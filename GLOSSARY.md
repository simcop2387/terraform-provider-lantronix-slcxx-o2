# Glossary

Vocabulary for the Lantronix SecureLinx console-server provider. Terms here are canonical; avoid drifted synonyms.

## Domain

**SecureLinx Console Manager (SLC)**
The Lantronix serial-console server family this provider manages: SLC08, SLC16, SLC24, SLC48. Configured through an interactive CLI session.

**Device Port**
A physical serial port on an SLC (one of up to 48). Its *port settings* are the electrical/serial parameters: baud rate, data bits, parity, stop bits, flow control.

**Connection Settings**
How a Device Port is reached over the network: which access services it exposes (SSH, Telnet, RFC 2217) and their per-port parameters. Distinct from the port's serial parameters.

**Transport**
The session channel through which the provider speaks the SLC CLI: Telnet (MVP), SSH or serial console (later). All transports carry the same CLI dialect.

**Session**
One authenticated CLI conversation: dial, login, commands, `logout`. Created per operation and always logged out — never pooled. The device permits only one live Session per user per transport, so Sessions are serialized.

**Pager Prompt**
The firmware's mid-output pause string (` -- Press <space> for next page... --`) emitted when `terminallines` is enabled. The Transport drives it away transparently; callers must never see it.

**Lab Device**
The single SLC48 at 192.168.0.197 (firmware 5.2a, single-bank) used for live development and acceptance. Its identity and credentials live in `secrets.txt`, never in the repo.

## This effort

**MVP**
The telnet-transport provider slice: manage Device Port settings and Connection Settings, with per-port Terraform import. Accepted under both `terraform` and OpenTofu against the Lab Device.

**Guardrails**
The inviolable live-device rules: never modify the Telnet service configuration, the network configuration, or the `sysadmin` password. All other device state is fair game for testing; additional users may be created and manipulated.
