package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeDevice speaks the SLC48 5.2a wire dialect as recorded in research
// issue #5: CR+NUL before "login:", server-side echo (never the password),
// prompt "[host]> " with trailing space, re-emitted on empty input, optional
// pager, logout banner then close.
type fakeDevice struct {
	t          *testing.T
	ln         net.Listener
	user, pass string
	promptHost string
	badFirst   int // reject this many login attempts before accepting
	lockout    bool
	pager      bool // page every command output once
}

func serveFake(t *testing.T, f *fakeDevice) *fakeDevice {
	t.Helper()
	f.t = t
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.ln = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.handle(c)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *fakeDevice) addr() (string, int) {
	h, p, _ := net.SplitHostPort(f.ln.Addr().String())
	var port int
	_, _ = fmt.Sscanf(p, "%d", &port)
	return h, port
}

func (f *fakeDevice) handle(c net.Conn) {
	defer func() { _ = c.Close() }()
	w := func(s string) { _, _ = c.Write([]byte(s)) }

	// IAC proposal like the device sends; client must answer and still work.
	_, _ = c.Write([]byte{iac, do, 24, iac, will, 3})

	w("\r\n\n\r\n\nWelcome to the SLC\n\n\n\n")
	w("\r\x00login: ")

	br := newLineReader(c)

	authed := false
	for attempt := 0; attempt < 5 && !authed; attempt++ {
		user, ok := br.next()
		if !ok {
			return
		}
		w(user + "\r\nPassword: ") // server echoes username, never password
		pass, ok := br.next()
		if !ok {
			return
		}
		w("\n\r\n")

		if f.lockout {
			w(lockoutText + "\r\n")
			time.Sleep(50 * time.Millisecond)
			return
		}
		switch {
		case user == f.user && pass == f.pass:
			authed = true
		default:
			w(badLoginText + "\r\n") // real device re-prompts, does not close
			w("login: ")
		}
	}
	if !authed {
		w("\r\nLogging out...\r\n")
		return
	}

	prompt := "[" + f.promptHost + "]> "
	w("\r\nWelcome to the SecureLinx Console Manager\r\nModel Number: SLC48\r\n\r\n" + prompt)
	for {
		cmd, ok := br.next()
		if !ok {
			return
		}
		if cmd == "" {
			w("\r\n" + prompt) // empty CR re-emits prompt
			continue
		}
		w(cmd + "\r\n") // echo
		switch cmd {
		case "logout":
			w("Logging out...\r\n\r\n")
			time.Sleep(20 * time.Millisecond)
			return
		case "slow":
			time.Sleep(3 * time.Second)
			w(prompt)
		case "drop":
			return // FIN mid-response
		default:
			out := "Baud Rate: 9600\r\nData Bits: 8\r\n"
			if f.pager {
				w(out + " -- Press <space> for next page, <enter> for next line, or 'q' to quit --")
				// wait for the space, then finish
				b := make([]byte, 1)
				for {
					n, err := c.Read(b)
					if n == 1 && b[0] == ' ' {
						break
					}
					if err != nil {
						return
					}
				}
				w("Parity: none\r\n" + prompt) // page two: new content only
				continue
			}
			w(out + "\r\n" + prompt)
		}
	}
}

// lineReader accumulates text (ignoring IAC sequences and NULs) and yields
// CR/LF-terminated lines.
type lineReader struct {
	c net.Conn
	b []byte
}

func newLineReader(c net.Conn) *lineReader { return &lineReader{c: c} }

func (lr *lineReader) next() (string, bool) {
	buf := make([]byte, 512)
	for {
		if i := indexLineEnd(lr.b); i >= 0 {
			line := string(stripIACNUL(lr.b[:i]))
			lr.b = lr.b[i+1:]
			return strings.TrimRight(line, "\r"), true
		}
		n, err := lr.c.Read(buf)
		if n > 0 {
			lr.b = append(lr.b, buf[:n]...)
		}
		if err != nil {
			return "", false
		}
	}
}

func indexLineEnd(b []byte) int {
	for i, x := range b {
		if x == '\n' {
			return i
		}
	}
	return -1
}

func stripIACNUL(b []byte) []byte {
	var out []byte
	for i := 0; i < len(b); {
		if b[i] == iac && i+2 < len(b) {
			i += 3
			continue
		}
		if b[i] != 0 {
			out = append(out, b[i])
		}
		i++
	}
	return out
}

func testCfg(f *fakeDevice) Config {
	h, p := f.addr()
	return Config{Host: h, Port: p, Username: "sysadmin", Password: "PASS", CommandTimeout: 2 * time.Second}
}

func TestRunHappyPath(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv"})
	tr := Dial(testCfg(f))
	out, err := tr.Run(context.Background(), "show deviceport port 47 display data")
	if err != nil {
		t.Fatal(err)
	}
	want := "Baud Rate: 9600\nData Bits: 8"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestRunDiscoversArbitraryHostname(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "slc36eb"})
	if _, err := Dial(testCfg(f)).Run(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
}

func TestLoginRetrySucceeds(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv", badFirst: 1})
	if _, err := Dial(testCfg(f)).Run(context.Background(), "x"); err != nil {
		t.Fatalf("retry should have recovered: %v", err)
	}
}

func TestAuthFailedAfterRetries(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "other", promptHost: "serserv"})
	cfg := testCfg(f)
	cfg.LoginRetries = 1
	_, err := Dial(cfg).Run(context.Background(), "x")
	var af *AuthFailedError
	if !errors.As(err, &af) {
		t.Fatalf("want AuthFailedError got %v", err)
	}
}

func TestLockoutStopsImmediately(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "u", pass: "p", promptHost: "serserv", lockout: true})
	start := time.Now()
	_, err := Dial(testCfg(f)).Run(context.Background(), "x")
	var af *AuthFailedError
	if !errors.As(err, &af) {
		t.Fatalf("want AuthFailedError got %v", err)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("lockout must stop after first attempt, took %v (retried)", d)
	}
}

func TestPagerDriven(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv", pager: true})
	out, err := Dial(testCfg(f)).Run(context.Background(), "show sysconfig")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, pagerMarker) || out != "Baud Rate: 9600\nData Bits: 8\nParity: none" {
		t.Fatalf("pager text leaked or output wrong: %q", out)
	}
}

func TestTimeoutAndReadRetry(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv"})
	cfg := testCfg(f)
	tr := Dial(cfg)
	_, err := tr.Run(context.Background(), "slow")
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("want TimeoutError got %v", err)
	}
}

func TestDisconnectedTyped(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv"})
	_, err := Dial(testCfg(f)).Run(context.Background(), "drop")
	var de *DisconnectedError
	if !errors.As(err, &de) {
		t.Fatalf("want DisconnectedError got %v", err)
	}
}

func TestMenuModeRejected(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "menu", pass: "p", promptHost: "Enter 1-4"})
	cfg := testCfg(f)
	cfg.Username = "menu"
	cfg.Password = "p"
	_, err := Dial(cfg).Run(context.Background(), "x")
	var af *AuthFailedError
	if !errors.As(err, &af) || !strings.Contains(err.Error(), "full-CLI") {
		t.Fatalf("want full-CLI AuthFailedError got %v", err)
	}
}

func TestConcurrentRunsSerialize(t *testing.T) {
	f := serveFake(t, &fakeDevice{user: "sysadmin", pass: "PASS", promptHost: "serserv"})
	tr := Dial(testCfg(f))
	errc := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { _, err := tr.Run(context.Background(), "x"); errc <- err }()
	}
	for i := 0; i < 8; i++ {
		if err := <-errc; err != nil && !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
	}
}
