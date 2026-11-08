package slc

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mustRead(t *testing.T, name string) string {
	t.Helper()
	b, err := testdataFS.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseSerial(t *testing.T) {
	got, err := ParseSerial(mustRead(t, "data_port47.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := Serial{Number: 47, Name: "Port-47", Baud: 9600, DataBits: 8, StopBits: 1,
		Parity: "none", FlowControl: "none", Logins: false}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestParseAccess(t *testing.T) {
	got, err := ParseAccess(mustRead(t, "ip_port1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := Access{Port: 1, TelnetEnabled: true, TelnetAuth: false, TelnetPort: 2000,
		SSHEnabled: false, SSHAuth: true, SSHPort: 3000,
		TCPEnabled: false, TCPAuth: false, TCPPort: 4000}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestParseIdleTimeout(t *testing.T) {
	n, err := ParseIdleTimeout(mustRead(t, "modem_port1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 30 {
		t.Fatalf("want 30 got %d", n)
	}
	if n, err := ParseIdleTimeout("Idle Timeout: disabled"); err != nil || n != 0 {
		t.Fatalf("disabled: want 0,nil got %d,%v", n, err)
	}
}

func TestParseRejectsMultiPort(t *testing.T) {
	out := mustRead(t, "data_port47.txt") + mustRead(t, "data_port47.txt")
	if _, err := ParseSerial(out); err == nil || !strings.Contains(err.Error(), "multi-port") {
		t.Fatalf("want multi-port rejection, got %v", err)
	}
}

func TestSetCommands(t *testing.T) {
	cmd := SetCmd(47, SetSerialParams(Serial{Baud: 115200, DataBits: 8, StopBits: 1, Parity: "none", FlowControl: "rts/cts", Logins: true}))
	want := "set deviceport port 47 baud 115200 databits 8 parity none stopbits 1 flowcontrol rts/cts logins enable"
	if cmd != want {
		t.Fatalf("got %q want %q", cmd, want)
	}
	got := DisableAccessCmd(48)
	want = "set deviceport port 48 telnetin disable sshin disable tcpin disable"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCheckSuccess(t *testing.T) {
	if err := CheckSuccess("Device Port settings successfully updated."); err != nil {
		t.Fatal(err)
	}
	err := CheckSuccess("Invalid number of parameters.")
	if err == nil || !strings.Contains(err.Error(), "Invalid number of parameters.") {
		t.Fatalf("want verbatim error, got %v", err)
	}
}
