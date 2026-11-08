package provider_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/provider"
	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/slc"
	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/transport"
)

// Acceptance harness (issue #7 + owner rules from issue #10):
//   - TF_ACC=1 required; runs against the Lab Device only.
//   - Credentials via LANTRONIX_SLCXX_O2_{USERNAME,PASSWORD}; host via SLC_HOST.
//   - Destructive steps restricted to SLC_TEST_PORT (default 47) and
//     SLC_TEST_PORT2 (default 48) — disconnected ports on the lab unit.
//   - Guardrails: never Telnet service config, network config, or the
//     sysadmin/simcop2387 accounts.

func proto6Factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"lantronix-slcxx-o2": providerserver.NewProtocol6WithError(provider.New("test")()),
	}
}

func testPort(t *testing.T) int { return envInt(t, "SLC_TEST_PORT", 47) }

func testPort2(t *testing.T) int { return envInt(t, "SLC_TEST_PORT2", 48) }

func envInt(t *testing.T, k string, def int) int {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s not a number: %v", k, err)
	}
	return n
}

func providerConfig() string {
	return fmt.Sprintf(`
provider "lantronix-slcxx-o2" {
  host     = %q
  username = %q
  password = %q
}
`, os.Getenv("SLC_HOST"), os.Getenv("LANTRONIX_SLCXX_O2_USERNAME"), os.Getenv("LANTRONIX_SLCXX_O2_PASSWORD"))
}

// deviceClient dials the lab device directly for out-of-band verification.
func deviceClient(t *testing.T) *slc.Client {
	t.Helper()
	return slc.NewClient(transport.Dial(transport.Config{
		Host:           os.Getenv("SLC_HOST"),
		Username:       os.Getenv("LANTRONIX_SLCXX_O2_USERNAME"),
		Password:       os.Getenv("LANTRONIX_SLCXX_O2_PASSWORD"),
		CommandTimeout: 60 * time.Second,
	}))
}

func TestAccPort_lifecycle(t *testing.T) {
	n := testPort(t)
	base := providerConfig()

	r.Test(t, r.TestCase{
		ProtoV6ProviderFactories: proto6Factories(),
		Steps: []r.TestStep{
			{ // adopt-and-set at 9600 8N1
				Config: base + fmt.Sprintf(`
resource "lantronix-slcxx-o2_port" "t" {
  number       = %d
  baud_rate    = 9600
  data_bits    = 8
  parity       = "none"
  stop_bits    = 1
  flow_control = "none"
  logins       = false
}
`, n),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckResourceAttr("lantronix-slcxx-o2_port.t", "baud_rate", "9600"),
					verifySerial(t, n, 9600),
				),
			},
			{ // update baud; readback must round-trip
				Config: base + fmt.Sprintf(`
resource "lantronix-slcxx-o2_port" "t" {
  number       = %d
  baud_rate    = 115200
  data_bits    = 8
  parity       = "none"
  stop_bits    = 1
  flow_control = "none"
  logins       = false
}
`, n),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckResourceAttr("lantronix-slcxx-o2_port.t", "baud_rate", "115200"),
					verifySerial(t, n, 115200),
				),
			},
			{ // per-port import by bare number (#8)
				ResourceName:      "lantronix-slcxx-o2_port.t",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     strconv.Itoa(n),
			},
			{ // destroy = forget: device keeps the last applied value (#9)
				Config: base,
			},
		},
	})

	client := deviceClient(t)
	got, err := client.GetSerial(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if got.Baud != 115200 {
		t.Errorf("forget-on-destroy must leave device untouched; baud is %d, want 115200", got.Baud)
	}
}

func TestAccPortAccess_disableOnDestroy(t *testing.T) {
	n := testPort2(t)
	base := providerConfig()

	r.Test(t, r.TestCase{
		ProtoV6ProviderFactories: proto6Factories(),
		Steps: []r.TestStep{
			{ // open telnet-in with auth on the sacrificial port
				Config: base + fmt.Sprintf(`
resource "lantronix-slcxx-o2_port_access" "t" {
  port           = %d
  telnet_enabled = true
  telnet_auth    = true
  ssh_enabled    = false
  tcp_enabled    = false
}
`, n),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckResourceAttr("lantronix-slcxx-o2_port_access.t", "telnet_enabled", "true"),
					verifyTelnetEnabled(t, n, true),
				),
			},
			{ // import by bare port number
				ResourceName:      "lantronix-slcxx-o2_port_access.t",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     strconv.Itoa(n),
			},
			{ // destroy must close all three services (#9)
				Config: base,
			},
		},
	})

	client := deviceClient(t)
	got, err := client.GetAccess(context.Background(), n)
	if err != nil {
		t.Fatal(err)
	}
	if got.TelnetEnabled || got.SSHEnabled || got.TCPEnabled {
		t.Errorf("destroy must disable all services; got %+v", got)
	}
}

func verifySerial(t *testing.T, port, wantBaud int) r.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := deviceClient(t).GetSerial(context.Background(), port)
		if err != nil {
			return err
		}
		if got.Baud != wantBaud {
			return fmt.Errorf("device says baud %d, want %d", got.Baud, wantBaud)
		}
		return nil
	}
}

func verifyTelnetEnabled(t *testing.T, port int, want bool) r.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := deviceClient(t).GetAccess(context.Background(), port)
		if err != nil {
			return err
		}
		if got.TelnetEnabled != want {
			return fmt.Errorf("device telnet_in=%v, want %v", got.TelnetEnabled, want)
		}
		return nil
	}
}
