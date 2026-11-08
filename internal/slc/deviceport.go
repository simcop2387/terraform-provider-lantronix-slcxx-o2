// Package slc is the typed CLI dialect layer over a transport.Transport:
// Device Port command builders and show-output parsers for firmware 5.2a.
package slc

import (
	"fmt"
	"strconv"
	"strings"
)

// Serial holds the per-port serial parameters (display data section).
type Serial struct {
	Number      int
	Name        string
	Baud        int
	DataBits    int
	StopBits    int
	Parity      string // none|odd|even
	FlowControl string // none|xon/xoff|rts/cts
	Logins      bool
}

// Access holds the per-port network access parameters (display ip section,
// plus Idle Timeout from the modem section).
type Access struct {
	Port          int
	TelnetEnabled bool
	TelnetAuth    bool
	TelnetPort    int
	SSHEnabled    bool
	SSHAuth       bool
	SSHPort       int
	TCPEnabled    bool
	TCPAuth       bool
	TCPPort       int
	IdleTimeout   int // seconds; 0 means disabled
}

const successMarker = "Device Port settings successfully updated."

// parseKV splits command output into trimmed label/value pairs. Multi-port
// repeats are rejected: reads are scoped to managed ports one at a time (#9).
func parseKV(out string) (map[string]string, error) {
	kv := map[string]string{}
	headerSeen := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "___") || strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		if strings.HasPrefix(line, "Number: ") {
			headerSeen++
			if headerSeen > 1 {
				return nil, fmt.Errorf("multi-port output in single-port read (%d headers)", headerSeen)
			}
			fields := strings.Fields(line) // Number: 47 Name: Port-47
			if len(fields) >= 4 {
				kv["Number"] = fields[1]
				kv["Name"] = fields[3]
			}
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		label := strings.TrimSpace(line[:i])
		value := strings.TrimSpace(line[i+1:])
		if value == "<none>" {
			value = ""
		}
		kv[label] = value
	}
	if headerSeen == 0 {
		return nil, fmt.Errorf("no device port block in output: %q", firstLine(out))
	}
	return kv, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ParseSerial parses `show deviceport port <n> display data` output.
func ParseSerial(out string) (Serial, error) {
	kv, err := parseKV(out)
	if err != nil {
		return Serial{}, err
	}
	n, err := strconv.Atoi(kv["Number"])
	if err != nil {
		return Serial{}, fmt.Errorf("bad port number %q: %w", kv["Number"], err)
	}
	s := Serial{Number: n, Name: kv["Name"], Parity: kv["Parity"], FlowControl: kv["Flow Control"]}
	if s.Baud, err = atoi(kv["Baud Rate"]); err != nil {
		return s, fmt.Errorf("baud: %w", err)
	}
	if s.DataBits, err = atoi(kv["Data Bits"]); err != nil {
		return s, fmt.Errorf("databits: %w", err)
	}
	if s.StopBits, err = atoi(kv["Stop Bits"]); err != nil {
		return s, fmt.Errorf("stopbits: %w", err)
	}
	s.Logins = kv["Logins"] == "enabled"
	return s, nil
}

// ParseAccess parses `show deviceport port <n> display ip` output. Idle
// Timeout is absent there and must be merged from ParseIdleTimeout.
func ParseAccess(out string) (Access, error) {
	kv, err := parseKV(out)
	if err != nil {
		return Access{}, err
	}
	n, err := strconv.Atoi(kv["Number"])
	if err != nil {
		return Access{}, fmt.Errorf("bad port number %q: %w", kv["Number"], err)
	}
	a := Access{
		Port:          n,
		TelnetEnabled: kv["Telnet"] == "enabled",
		TelnetAuth:    kv["Telnet Auth"] == "enabled",
		SSHEnabled:    kv["SSH"] == "enabled",
		SSHAuth:       kv["SSH Auth"] == "enabled",
		TCPEnabled:    kv["TCP"] == "enabled",
		TCPAuth:       kv["TCP Auth"] == "enabled",
	}
	for _, f := range []struct {
		label string
		dest  *int
	}{
		{"Telnet Port", &a.TelnetPort}, {"SSH Port", &a.SSHPort}, {"TCP Port", &a.TCPPort},
	} {
		if *f.dest, err = atoi(kv[f.label]); err != nil {
			return a, fmt.Errorf("%s: %w", f.label, err)
		}
	}
	return a, nil
}

// ParseIdleTimeout extracts the Idle Timeout value ("30 seconds" or
// "disabled") from `show deviceport port <n> display modem` output.
func ParseIdleTimeout(out string) (int, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if !strings.HasPrefix(line, "Idle Timeout:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "Idle Timeout:"))
		if v == "disabled" || v == "" {
			return 0, nil
		}
		digits := strings.Fields(v)[0]
		n, err := strconv.Atoi(digits)
		if err != nil {
			return 0, fmt.Errorf("idle timeout %q: %w", v, err)
		}
		return n, nil
	}
	return 0, fmt.Errorf("no Idle Timeout line")
}

// SetSerialParams renders parameter tokens for a batched serial set command.
func SetSerialParams(want Serial) []string {
	var p []string
	add := func(k, v string) { p = append(p, k, v) }
	if want.Baud != 0 {
		add("baud", strconv.Itoa(want.Baud))
	}
	if want.DataBits != 0 {
		add("databits", strconv.Itoa(want.DataBits))
	}
	if want.Parity != "" {
		add("parity", want.Parity)
	}
	if want.StopBits != 0 {
		add("stopbits", strconv.Itoa(want.StopBits))
	}
	if want.FlowControl != "" {
		add("flowcontrol", want.FlowControl)
	}
	add("logins", boolWord(want.Logins))
	if want.Name != "" {
		add("name", want.Name)
	}
	return p
}

// SetAccessParams renders parameter tokens for a batched access set command.
func SetAccessParams(want Access) []string {
	p := []string{
		"telnetin", boolWord(want.TelnetEnabled), "telnetauth", boolWord(want.TelnetAuth),
		"sshin", boolWord(want.SSHEnabled), "sshauth", boolWord(want.SSHAuth),
		"tcpin", boolWord(want.TCPEnabled), "tcpauth", boolWord(want.TCPAuth),
	}
	if want.TelnetPort != 0 {
		p = append(p, "telnetport", strconv.Itoa(want.TelnetPort))
	}
	if want.SSHPort != 0 {
		p = append(p, "sshport", strconv.Itoa(want.SSHPort))
	}
	if want.TCPPort != 0 {
		p = append(p, "tcpport", strconv.Itoa(want.TCPPort))
	}
	idle := "disable"
	if want.IdleTimeout > 0 {
		idle = strconv.Itoa(want.IdleTimeout)
	}
	return append(p, "idletimeout", idle)
}

// DisableAccessCmd closes all three services on destroy of a port_access
// resource (#9: releasing management never leaves a door open).
func DisableAccessCmd(number int) string {
	return fmt.Sprintf("set deviceport port %d telnetin disable sshin disable tcpin disable", number)
}

func SetCmd(number int, params []string) string {
	return fmt.Sprintf("set deviceport port %d %s", number, strings.Join(params, " "))
}

func ShowDataCmd(number int) string {
	return fmt.Sprintf("show deviceport port %d display data", number)
}

func ShowIPCmd(number int) string {
	return fmt.Sprintf("show deviceport port %d display ip", number)
}

func ShowModemCmd(number int) string {
	return fmt.Sprintf("show deviceport port %d display modem", number)
}

// CheckSuccess reports whether a set command was accepted; returns the
// verbatim device error otherwise.
func CheckSuccess(out string) error {
	if strings.Contains(out, successMarker) {
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "___") {
			return fmt.Errorf("device replied: %s", line)
		}
	}
	return fmt.Errorf("device returned no confirmation")
}

func boolWord(b bool) string {
	if b {
		return "enable"
	}
	return "disable"
}

func atoi(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("bad number %q", s)
	}
	return n, nil
}
