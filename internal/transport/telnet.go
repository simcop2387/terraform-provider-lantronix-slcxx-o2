package transport

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	iac, will, wont, do, dont, sb, se = 255, 251, 252, 253, 254, 250, 240

	pagerMarker = "Press <space> for next page"
	maxPages    = 500

	promptRe      = `\[[^\]\r\n]+\]> \z`
	menuPromptRe  = `\[Enter 1-\d+\]> \z`
	badLoginText  = "Incorrect Login or Username"
	lockoutText   = "Too many incorrect login attempts."
	logoutBanner  = "Logging out..."
	cmdTerminator = "\r\n"
)

var (
	promptPat   = regexp.MustCompile(promptRe)
	menuPrompt  = regexp.MustCompile(menuPromptRe)
	loginPat    = regexp.MustCompile(`login: \z`)
	passwordPat = regexp.MustCompile(`Password: \z`)
)

// Dial returns a ready Transport. No session is opened until the first call;
// each operation runs its own dial-login-run-logout cycle, serialized by an
// internal mutex (the device refuses same-user sessions on one transport).
func Dial(cfg Config) Transport {
	return &telnet{cfg: cfg.withDefaults()}
}

type telnet struct {
	cfg Config
	mu  sync.Mutex // session serialization per credential (#6)
}

func (t *telnet) Run(ctx context.Context, command string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.session(ctx, command)
}

func (t *telnet) Read(ctx context.Context, command string) (string, error) {
	out, err := t.Run(ctx, command)
	if err != nil && retryable(err) {
		return t.Run(ctx, command)
	}
	return out, err
}

func (t *telnet) session(ctx context.Context, command string) (string, error) {
	addr := net.JoinHostPort(t.cfg.Host, fmt.Sprint(t.cfg.Port))
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", addr, err)
	}
	s := &session{conn: conn, cfg: t.cfg}
	defer s.logout()

	if err := s.login(ctx); err != nil {
		return "", err
	}
	out, err := s.exec(ctx, command)
	if err != nil {
		return "", err
	}
	return out, nil
}

// session is one authenticated CLI conversation over a conn. Wire bytes pass
// through an IAC filter: negotiation is refused uniformly (the SLC works in
// line mode with everything declined), NULs are stripped, and text lands in
// s.clean with chunk boundaries normalized away.
type session struct {
	conn  net.Conn
	cfg   Config
	clean bytes.Buffer // IAC-stripped text received since last reset
}

func (s *session) pump(deadline time.Time) error {
	if err := s.conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if n > 0 {
		s.consume(buf[:n])
	}
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil // caller decides via deadline check
		}
		return err
	}
	return nil
}

func (s *session) consume(in []byte) {
	var replies []byte
	for i := 0; i < len(in); {
		b := in[i]
		switch {
		case b == iac && i+1 < len(in) && in[i+1] == iac:
			s.clean.WriteByte(iac)
			i += 2
		case b == iac && i+2 < len(in) && in[i+1] == sb:
			j := i + 2
			for j+1 < len(in) && !(in[j] == iac && in[j+1] == se) {
				j++
			}
			i = min(j+2, len(in)) // whole SB..SE dropped
		case b == iac && i+2 < len(in):
			cmd, opt := in[i+1], in[i+2]
			switch cmd {
			case do, will:
				reject := byte(wont)
				if cmd == do {
					reject = dont
				}
				replies = append(replies, iac, reject, opt)
			}
			i += 3
		case b == iac:
			i++ // incomplete IAC at chunk edge; wait for more
		case b == 0:
			i++ // pre-login CR+NUL quirk
		default:
			s.clean.WriteByte(b)
			i++
		}
	}
	if len(replies) > 0 {
		_, _ = s.conn.Write(replies)
	}
}

func (s *session) waitFor(ctx context.Context, re *regexp.Regexp, fixed ...string) error {
	deadline := time.Now().Add(s.cfg.CommandTimeout)
	for {
		text := s.clean.String()
		if re != nil && re.MatchString(text) {
			return nil
		}
		for _, f := range fixed {
			if strings.Contains(text, f) {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return &TimeoutError{Command: "<handshake>", After: s.cfg.CommandTimeout}
		}
		if err := s.pump(min2(deadline, time.Now().Add(2*time.Second))); err != nil {
			return &DisconnectedError{Err: err}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (s *session) login(ctx context.Context) error {
	attempts := 1 + s.cfg.LoginRetries
	var lastErr error
	for a := 0; a < attempts; a++ {
		s.clean.Reset()
		if err := s.waitFor(ctx, loginPat); err != nil {
			return err
		}
		if _, err := s.conn.Write([]byte(s.cfg.Username + cmdTerminator)); err != nil {
			return &DisconnectedError{Err: err}
		}
		s.clean.Reset()
		if err := s.waitFor(ctx, passwordPat); err != nil {
			return err
		}
		if _, err := s.conn.Write([]byte(s.cfg.Password + cmdTerminator)); err != nil {
			return &DisconnectedError{Err: err}
		}
		s.clean.Reset()
		if err := s.waitFor(ctx, promptPat, badLoginText, lockoutText); err != nil {
			return err
		}
		text := s.clean.String()
		switch {
		case strings.Contains(text, lockoutText):
			return &AuthFailedError{Err: fmt.Errorf("device reports %q; stopping to avoid extended lockout", lockoutText)}
		case strings.Contains(text, badLoginText):
			lastErr = fmt.Errorf("device reported %q", badLoginText)
			continue
		case menuPrompt.MatchString(text):
			return &AuthFailedError{Err: fmt.Errorf("account lands in menu mode; a full-CLI admin user is required")}
		case promptPat.MatchString(text):
			return nil
		default:
			return fmt.Errorf("unexpected post-login output: %q", tail(text, 120))
		}
	}
	return &AuthFailedError{Err: lastErr}
}

func (s *session) exec(ctx context.Context, command string) (string, error) {
	s.clean.Reset()
	if _, err := s.conn.Write([]byte(command + cmdTerminator)); err != nil {
		return "", &DisconnectedError{Err: err}
	}
	pages := 0
	deadline := time.Now().Add(s.cfg.CommandTimeout)
	for {
		text := s.clean.String()
		if promptPat.MatchString(text) {
			return stripEcho(text, command), nil
		}
		if strings.Contains(tailBytes(text, 160), pagerMarker) {
			pages++
			if pages > maxPages {
				return "", fmt.Errorf("pager ran past %d pages; aborting", maxPages)
			}
			if _, err := s.conn.Write([]byte(" ")); err != nil {
				return "", &DisconnectedError{Err: err}
			}
		}
		if time.Now().After(deadline) {
			return "", &TimeoutError{Command: command, After: s.cfg.CommandTimeout}
		}
		if err := s.pump(min2(deadline, time.Now().Add(2*time.Second))); err != nil {
			return "", &DisconnectedError{Err: err}
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
}

// stripEcho removes the server's echo of the command line and the trailing
// prompt, leaving payload lines with per-line trailing spaces trimmed.
func stripEcho(text, command string) string {
	text = strings.TrimPrefix(text, "\r\n")
	if i := strings.Index(text, "\n"); i >= 0 {
		line := strings.TrimRight(text[:i], "\r")
		if strings.TrimSpace(line) == strings.TrimSpace(command) {
			text = text[i+1:]
		}
	}
	if m := promptPat.FindStringIndex(text); m != nil {
		text = text[:m[0]]
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(text, "\r\n"), "\n") {
		if strings.Contains(line, pagerMarker) {
			continue // pager prompt absorbed by the Transport (#6)
		}
		kept = append(kept, strings.TrimRight(line, " \t\r"))
	}
	return strings.Join(kept, "\n")
}

func (s *session) logout() {
	if _, err := s.conn.Write([]byte("logout" + cmdTerminator)); err != nil {
		_ = s.conn.Close()
		return
	}
	_ = s.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 256)
	for {
		n, err := s.conn.Read(buf)
		if n == 0 || err != nil {
			break
		}
		if bytes.Contains(buf[:n], []byte(logoutBanner)) {
			break
		}
	}
	_ = s.conn.Close()
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func tailBytes(s string, n int) string { return tail(s, n) }

func min2(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
