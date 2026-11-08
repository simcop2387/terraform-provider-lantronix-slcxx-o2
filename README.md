# terraform-provider-lantronix-slcxx-o2

Terraform/OpenTofu provider for **Lantronix SecureLinx SLCxx-O2** serial console
servers (SLC08/16/24/48, firmware 5.x), driving the device's own CLI over a
telnet session. See ADR 0001: the same CLI dialect later rides SSH and the
serial console, and works even when the device's network config is broken.

**Status: telnet MVP in development.** Resources, import, lifecycle semantics,
and acceptance harness are implemented; the live acceptance run against the lab
device is pending (see the [wayfinder map](https://github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/issues/1)).

## Quick start

```hcl
terraform {
  required_providers {
    lantronix-slcxx-o2 = { source = "simcop2387/lantronix-slcxx-o2" }
  }
}

provider "lantronix-slcxx-o2" {
  host     = "192.168.0.197"
  username = "sysadmin"        # or LANTRONIX_SLCXX_O2_USERNAME
  password = var.slc_password  # or LANTRONIX_SLCXX_O2_PASSWORD
}

resource "lantronix-slcxx-o2_port" "router_console" {
  number       = 47
  baud_rate    = 115200
  data_bits    = 8
  parity       = "none"
  stop_bits    = 1
  flow_control = "none"
  logins       = false
}

resource "lantronix-slcxx-o2_port_access" "router_console" {
  port           = 47
  telnet_enabled = true
  telnet_auth    = true
}

# Import an already-configured port by its bare number:
#   terraform import lantronix-slcxx-o2_port.router_console 47
```

Provider name `lantronix-slcxx-o2` (dashes, not underscores — Terraform and
OpenTofu reject underscores in provider local names; see ADR 0002 for why the
name pins the O2 generation).

## Semantics that matter

- **Ports exist by hardware.** Creating a resource *adopts* an existing port and
  applies only declared attributes (unspecified ones are preserved). Destroying
  `..._port` only forgets it; destroying `..._port_access` **disables Telnet/SSH/TCP
  In** on the port so releasing management never leaves a door open.
- Every `set` is batched per resource and always followed by a readback from the
  device — state is what the device says, not what we sent.
- Persistence is automatic on this firmware; the provider never invokes
  `admin config save`.
- Sessions are one-login-per-operation with mandatory logout, serialized per
  credential (the device refuses concurrent same-user CLI sessions).

## Development

```
make build     # -> dist/terraform-provider-lantronix-slcxx-o2
make test      # unit tests (parser goldens + fake-device transport)
make lint
make smoke     # terraform plan AND tofu plan via dev_overrides, no device needed
make acc-tf    # acceptance against the lab device under HashiCorp Terraform
make acc-tofu  # same suite under OpenTofu
```

Acceptance tests require `TF_ACC=1` plus `SLC_HOST`, `LANTRONIX_SLCXX_O2_USERNAME`,
`LANTRONIX_SLCXX_O2_PASSWORD` and mutate **only** ports `SLC_TEST_PORT`/`SLC_TEST_PORT2`
(default 47/48 — disconnected ports on the lab unit). Never point them at a device
with load-bearing consoles attached.

### Lab guardrails (non-negotiable)

Never modify: the Telnet **service** config, network configuration (recovery
needs the front-panel console), or the `sysadmin`/`simcop2387` accounts.

### Known device hazard

Ghosted telnet sessions (killed clients leaving half-open slots) accumulate in
firmware 5.2a's login daemon until it refuses **all** new connections — telnet,
SSH, and web alike — recoverable only by rebooting the device. The provider
mitigates this with pager-quitting, always-on logout; development scripts must
log out cleanly too.

## Layout

    main.go                 provider server entry
    internal/transport/     session mechanics: IAC/TTYPE login, prompt engine, pager, typed errors
    internal/slc/           Device Port dialect: parsers (golden-tested) + batched set builders
    internal/provider/      terraform-plugin-framework provider, port + port_access resources
    examples/               smoke config for dev_overrides plans
    docs/                   generated schema docs (`make docs`) + ADRs/agent docs
