# Research: SLC CLI surface for Connection Settings (per-port access services)

Resolves issue #3. Question (verbatim):

> exact CLI command surface for Connection Settings — how each Device Port on a Lantronix
> SLC48 (firmware 5.2a) is reached over the network: which access services exist per port
> (SSH, Telnet, RFC 2217, others?) and commands to show/set their enablement per port;
> service TCP port numbers and how they map to device ports and whether per-port configurable;
> other per-port connection parameters (raw vs cook mode, connect-back, etc.) that belong in
> Connection Settings; exact verbatim output format of relevant show commands for parser design.

Method: primary sources only — the vendor user guide (`vendor_firmware/SLC_UG.pdf`, extracted
with `pdftotext -layout`), the on-device web help shipped in the 5.2a firmware image
(`vendor_firmware/ext/slcupdate-5.2a/www/help*/`), the firmware's internal XML config model
(`www/xml-defaults/devports.{dtd,xml}`), and a live read-only Telnet session against
192.168.0.197 (SLC48, firmware 5.2a). All device interaction was `show`/`help` only; nothing
was modified.

## TL;DR answer

- Per-port network access services on SLC 5.2a are exactly **three**: **Telnet In**, **SSH In**,
  and **TCP In** (raw TCP). There is **no RFC 2217** support (see Surprises). Each has an
  enable flag and a separate per-service authentication flag.
- Modeled in the CLI as three parameter triples on `set deviceport port`:
  `telnetin|telnetport|telnetauth`, `sshin|sshport|sshauth`, `tcpin|tcpport|tcpauth`.
- Port numbers: each device port N gets a Telnet/SSH/TCP listener number derived from three
  **global** starting ports (`set deviceport global telnetport|sshport|tcpport`), assigned as
  `starting + (N - 1)`; any per-port value can be overridden with the per-port `*port` params.
- A device port may also own a **dedicated IP address** (`ipaddr`); then Telnet/SSH connect to
  that IP on standard ports 23/22, raw TCP on the port's TCP listener number.
- "Raw vs cook" exists but not under those names: Telnet In / SSH In are the cooked paths
  (login auth, banner, break sequence); TCP In is the raw path (no auth by default; vendor tells
  you to clear `breakseq` for raw/binary use).
- The parser's best friend is `show deviceport port <N> display ip` — single-column, one
  `Label: value` per line, stable field set. Full `show deviceport port` output is a fixed
  three-column layout that is painful to parse.

## 1. Access services per port

From the user guide, "Connection Methods" (UG p. 89–90) and "Device Ports - Settings → IP
Settings" (UG p. 96), confirmed by on-device web help `www/help/Device_Port_Settings.htm`
("Telnet in — Enables access to this port through Telnet", same for SSH / raw TCP) and the live
device:

| Service | Enable param | Auth param | Listener param | Show label (5.2a) | Default (factory XML) |
|---|---|---|---|---|---|
| Telnet In | `telnetin <enable\|disable>` | `telnetauth <enable\|disable>` | `telnetport <TCP Port>` | `Telnet`, `Telnet Port`, `Telnet Auth` | disabled, auth enabled |
| SSH In | `sshin <enable\|disable>` | `sshauth <enable\|disable>` | `sshport <TCP Port>` | `SSH`, `SSH Port`, `SSH Auth` | disabled, auth enabled |
| TCP In (raw) | `tcpin <enable\|disable>` | `tcpauth <enable\|disable>` | `tcpport <TCP Port>` | `TCP`, `TCP Port`, `TCP Auth` | disabled, auth disabled |

Factory defaults per `www/xml-defaults/devports.xml`: `TELNET_ENABLED=n`, `TELNET_AUTH=y`,
`SSH_ENABLED=n`, `SSH_AUTH=y`, `TCP_ENABLED=n`, `TCP_AUTH=n`. (UG p. 96: "Authenticate is
selected by default for Telnet in and SSH in, but not for TCP in.")

There are **no other per-port inbound network services**. Everything else network-shaped is
either system-wide (`set services ssh|telnet|webssh|webtelnet|portssh …` controls CLI logins to
the management interface, not device ports) or outbound (`connect …`, §4).

Per-port authentication semantics: with auth enabled the user logs into the SLC before being
bridged to the serial stream; with auth disabled the connection is bridged straight through
(UG p. 90, "if logins or authentication are not enabled, the user is directly connected to the
device port with no authentication").

## 2. Show/set commands (5.2a on-device synopsis, verbatim)

Captured live from `[serserv]> help set deviceport` (this is the authoritative 5.2a surface):

```text
Device Port Command Synopsis
----------------------------
set deviceport global <one or more parameters>
Parameters:  telnetport <TCP Port>
             sshport <TCP Port>
             tcpport <TCP Port>
             maxdirect <1-10>

set deviceport port <Device Port # or List or Name> <one or more parameters>
Parameters:                         name <Device Port Name>
telnetin <enable|disable>           banner <Banner Text>
telnetport <TCP Port>               device <none|slp8|slp16>
telnetauth <enable|disable>         breakseq <1-10 Chars>
sshin <enable|disable>              modemstate <disable|dialin|dialout|dialback|
sshport <TCP Port>                              dialondemand|dialin+ondemand>
sshauth <enable|disable>            modemmode <text|ppp>
tcpin <enable|disable>              localipaddr <negotiate|IP Address>
tcpport <TCP Port>                  remoteipaddr <negotiate|IP Address>
tcpauth <enable|disable>            auth <pap|chap>
ipaddr <IP Address>                 chaphost <CHAP Host or User Name>
webcolumns <Web SSH/Telnet Cols>    chapsecret <CHAP Secret or User Password>
webrows <Web SSH/Telnet Rows>       dodauth <pap|chap>
baud <300-115200>                   dodchaphost <CHAP Host or User Name>
databits <7|8>                      dodchapsecret <CHAP Secret or User Password>
stopbits <1|2>                      initscript <Modem Initialization Script>
parity <none|odd|even>              timeoutlogins <disable|1-30 minutes>
flowcontrol <none|xon/xoff|rts/cts> dialoutnumber <Phone Number>
logins <enable|disable>             dialoutlogin <User Login>
checkdsr <enable|disable>           dialoutpassword <Password>
closedsr <enable|disable>           dialbacknumber <usernumber|Phone Number>
locallogging <enable|disable>       nat <enable|disable>
nfslogging <enable|disable>         idletimeout <disable|1-9999 seconds>
nfsmaxfiles <Max # of Files>        restartdelay <PPP Restart Delay>
nfsmaxsize <Size in Bytes>          calleridlogging <enable|disable>
nfsdir <Logging Directory>          calleridcmd <Modem Command String>
pccardlogging <enable|disable>      emaillogging <disable|bytecnt|charstr>
pccardmaxfiles <Max # of Files>     emailsend <email|trap|both>
pccardmaxsize <Size in Bytes>       emailstring <Regex String>
pcccardslot <upper|lower>           emailthreshold <Threshold>
slmlogging <enable|disable>         emaildelay <Email Delay>
slmnms <NMS IP Address>             emailrestart <Restart Delay>
slmthreshold <Threshold>            emailto <Email Address>
slmtime <Time Frame>                emailsubj <Email Subject>
sysloglogging <enable|disable>

Note: A group of device ports can be configured by specifying a
      comma-separated list of ports (i.e., '1-4,8,10-12').
      Remove breakseq for Device Ports connected to raw binary connections.
      The logging level for the Device Ports log must be set to 'Info'
      to view Syslog entries for Device Port logging.
      It is recommended that the initscript be prepended with 'AT' and include
      'E1 V1 x4 Q0' so that the SLC may properly control the modem.

show deviceport global
show deviceport names
show deviceport port <Device Port List or Name>
                [display <ip|data|modem|logging|device>]
```

The device-port list argument accepts numbers, ranges and comma lists (`1-4,8,10-12`) and — per
UG p. 104/273 — also `'ALL'` and port names.

Also relevant, from UG ch. 7 (p. 74) for the **system-wide** login services that gate CLI
access (not per-port): `set services ssh|telnet|webssh|webtelnet|v1ssh <enable|disable>`,
`portssh <TCP Port>` (management SSH port), `timeoutssh|timeouttelnet <disable|1-30>`, and
`show services`.

## 3. TCP port number mapping

UG p. 93 ("Telnet/SSH/TCP in Port Numbers") + live device:

- Three **global** starting ports, one per service: Starting Telnet Port (default base gives
  `2000 + N`), Starting SSH Port (`3000 + N`), Starting TCP Port (`4000 + N`). Valid range
  1025–65528; ports 1–1024 are to be avoided (conflict with system services).
- Assignment is `starting + (port_number - 1)`. The factory XML defaults ship
  `START_TELNET_PORT=2001`, `START_SSH_PORT=3001`, `START_TCP_PORT=4001` — i.e. port N lands on
  2000+N / 3000+N / 4000+N. The lab unit was reconfigured to start at 2000/3000/4000, so its
  port 1 sits on 2000/3000/4000 (see transcript).
- Per-port override: `set deviceport port <N> telnetport|sshport|tcpport <P>` overrides the
  derived number for that one port ("Automatically assigned … You may override this value" —
  UG p. 96).
- Per-port dedicated IP (`ipaddr`): if set, clients can Telnet/SSH to *that IP* on standard
  ports 23/22 and land directly on the device port; raw TCP still uses the port's TCP listener
  number (UG p. 90 #5, p. 96). Netmask bits are a web-UI-only suffix concept (`x.x.x.x/24`);
  the CLI param is bare `ipaddr <IP Address>`; clear with `0.0.0.0`.
- Raw TCP listeners only exist when `tcpin` is enabled (UG p. 96 "Disabled by default"); same
  gating logic for telnet/ssh-in listener numbers shown by `show deviceport port` regardless of
  enablement — the number is displayed even when the service is disabled, so parser must read
  both lines.

## 4. Other per-port connection parameters belonging in "Connection Settings"

From the on-device synopsis above + UG ch. 8/11:

- `ipaddr` — dedicated IPv4 for the port (see §3). Bound to Eth1 only; unsupported with
  Ethernet bonding (UG p. 97 note).
- Per-service auth flags (`telnetauth`, `sshauth`, `tcpauth`) — login-or-bridge decision.
- `idletimeout <disable|1-9999 seconds>` — idle disconnect for Telnet/SSH/TCP connections to
  the port (web "Timeout" 1–1800 s per UG p. 97; CLI range is wider).
- `logins <enable|disable>` — show an SLC login prompt on the *serial* side; must be disabled
  for the port to act as a network endpoint (UG p. 97, ch. 11 note at p. 161).
- `maxdirect <1-10>` — simultaneous direct connections per port. **On 5.2a this is global**
  (`set deviceport global maxdirect`, shown as "Maximum # of Direct Connects per Device Port"
  in `show deviceport global`; the internal model has `GLOBAL/MAX_DIRECT`). The UG Rev F
  appendix lists it under per-port params — that drifts from 5.2a reality (see Surprises).
- `banner`, `breakseq` — shown to/interpreted for network users of the port. Vendor note:
  clear `breakseq` ("Remove breakseq for Device Ports connected to raw binary connections") so
  binary payloads aren't intercepted by the escape sequence — this is the practical
  "raw vs cooked" knob alongside choosing TCP In over Telnet/SSH In.
- `webcolumns`/`webrows` — geometry for the Java Web SSH/Telnet applet per port.
- DSR gating: `checkdsr`, `closedsr` (connect only if DSR asserted / drop on DSR loss).
- Connect-back: in 5.2a this is modem **dial-back** (`modemstate dialback`, `dialbacknumber
  <usernumber|Phone Number>`, PPP CBCP per UG ch. 8) — there is no network-level connect-back.
- Outbound "connections" (UG ch. 11, p. 157–164) pair a device port with an endpoint:
  `connect bidirection <port> <endpoint>` / `connect unidirection <port> dataflow <dir>
  <endpoint>` where endpoint ∈ `deviceport | telnet <host> [port P] | ssh <host> [port P]
  [user|version|command] | tcp <host> [port P] | udp <host> [port P]`, with triggers
  (`trigger now|datetime|chars`, `date`, `charcount|charseq|charxfer`) and
  `exclusive <enable|disable>`; global outgoing idle timeout via
  `connect global outgoingtimeout <disable|1-9999 seconds>`; teardown via
  `connect terminate <Connection ID>`; view via `show connections [connid <ID>]`. This is the
  SLC's "reverse telnet"/terminal-server feature set — likely a separate resource from
  per-port access services.

## 5. Verbatim show-command output (firmware 5.2a, live device)

Captured over Telnet to `[serserv]>` prompt; reproduced byte-for-byte except CRLF→LF and the
trailing prompt line. Note trailing spaces after some values ("SSH Auth: enabled ").

### `show services` (system-wide login services — context for Connection Settings)

```text
___Current Services Settings___________________________________________________
System Logging Levels: Network: Warning      Authentication: Warning
                       Services: Warning     Device Ports: Warning
                       General: Warning      Diagnostics: Warning
Remote Syslog Server #1: <none>
Remote Syslog Server #2: <none>
Audit Logging: enabled                       Audit Log Size: 50 Kbytes
Include CLI Commands in Log: disabled        Include In System Log: disabled
SSH Logins: enabled                          SSH Timeout: disabled
Web SSH: enabled                             SSH Port: 22
SSH Version 1 Logins: enabled
Telnet Logins: enabled                       Telnet Timeout: disabled
Web Telnet: enabled
SNMP Agent: disabled
SNMP Traps: disabled                         SNMP NMS: <none>
SNMP Location: location                      SNMP Contact: contact
SNMP RO Community: public                    SNMP V3 User: snmpuser
SNMP RW Community: private                   SNMP V3 Password: ********
SNMP Trap Community: public                  SNMP Alarm Delay: 60 seconds
SMTP Server: <none>
Phone Home: disabled                         Last Attempt: N/A
Phone Home IP Address: <none>                Results: N/A
```

### `show deviceport global`

```text
___Current Device Port Global Settings_________________________________________
Starting Telnet Port: 2000
Starting SSH Port: 3000
Starting TCP Port: 4000
Maximum # of Direct Connects per Device Port: 2
```

### `show deviceport names` (48-port layout: two columns of 24)

```text
___Current Device Port Names___________________________________________________
 01 - Port-1                         25 - Port-25
 02 - Port-2                         26 - Port-26
 ...
 24 - Port-24                        48 - Port-48
```

### `show deviceport port 1 display ip` — **recommended parse target** (single column)

```text
___Current Device Port Settings________________________________________________
Number: 1  Name: Port-1
Banner: <none>

IP Settings--------------------------------------------------------------------
Telnet: enabled
Telnet Port: 2000
Telnet Auth: disabled
SSH: disabled
SSH Port: 3000
SSH Auth: enabled
TCP: disabled
TCP Port: 4000
TCP Auth: disabled
IP: <none>
Web Columns: 80
Web Rows: 24
```

### `show deviceport port 1-2` (full, three-column; per-port block repeats)

```text
___Current Device Port Settings________________________________________________
Number: 1  Name: Port-1
Banner: <none>

Modem Settings----------------------Data Settings----------IP Settings---------
Modem State: disabled               Baud Rate: 115200      Telnet: enabled
Modem Mode: Text                    Data Bits: 8           Telnet Port: 2000
Timeout Logins: disabled            Stop Bits: 1           Telnet Auth: disabled
Local IP: negotiate                 Parity: none           SSH: disabled
Remote IP: negotiate                Flow Control: none     SSH Port: 3000
Authentication: PAP                 Logins: disabled       SSH Auth: enabled
CHAP Host: <none>                   Check DSR: disabled    TCP: disabled
CHAP Secret: <none>                 Close DSR: disabled    TCP Port: 4000
DOD Authentication: PAP             Break Seq: \x1bB       TCP Auth: disabled
DOD CHAP Host: <none>                                      IP: <none>
DOD CHAP Secret: <none>                                    Web Columns: 80
NAT: disabled                                              Web Rows: 24
Idle Timeout: 30 seconds
Restart Delay: 30 seconds
Dial-out Number: <none>
Dial-out Login: <none>
Dial-out Password: <none>
Dial-back Number: usernumber
Initialization Script: <none>
Caller ID Logging: disabled
Caller ID Cmd: <none>

Logging Settings---------------------------------------------------------------
Email/Trap: disabled                Local Logging: disabled
Send via: Email                     PC Card Logging: disabled
Byte Threshold: 100                 Log to: Upper Slot
Email Delay: 60 seconds             Max Number of Files: 10
Restart Delay: 60 seconds           Max Size of Files: 2048
Text String: <none>
Email To: <none>
Email Subject: Port %d Logging
                                    SLM Logging: disabled
NFS File Logging: disabled          NMS: <none>
Max Number of Files: 10             Byte Threshold: 100
Max Size of Files: 2048             Time Frame: 60 seconds
Directory to log to: <none>
Syslog Logging: disabled

Device Settings----------------------------------------------------------------
Connected To: <none>
Login: <none>
Password: <none>
```

(the repeat of the block for port 2 shows 2001/3001/4001 — the derived numbering off the
global starting ports of 2000/3000/4000. Columns start at fixed character offsets: Data
Settings at offset 36, IP Settings at offset 59 (0-based) in the section header and value
rows.)

### `show portstatus deviceport 1-3` (runtime state, not config)

```text
Device Port:                     1      DSR/CD:                        Yes
Name:                       Port-1      DTR:                           Yes
Mode:                         Idle      CTS:                           Yes
                                        RTS:                           Yes

Device Port:                     2      DSR/CD:                         No
...
```

Values right-aligned in fixed columns; blank line between ports; no `___` banner.

### `show connections` (live sessions table)

```text
 Id Port/Service              Flow Port/Service              User       Uptime
=== ========================= ==== ========================= ========== ========
 53*Telnet In 192.168.1.146   <--> Command Line              <user>     00:00:14
 52 Telnet In 192.168.1.146   <--> Command Line                         00:00:34
  3 Console Port              <--> Command Line                         00:32:32
```

`*` marks the session you are in; `Flow` ∈ `<-->` etc.; connection IDs churn when a
connection restarts (UG p. 164 note).

### Parser design notes

- Section banners: `___Current <Thing>____…` (underscores) for config shows,
  `Label Settings----…` (hyphens) for sub-sections; strip and treat as record separators.
- Sentinel for empty values is `<none>`; booleans render as `enabled`/`disabled` in show output
  but are set as `enable`/`disable`.
- Some lines carry trailing spaces (`SSH Auth: enabled `); trim right before matching.
- The command is echoed back by the device (telnet echo) — strip the first line of each reply.
- Paging: output longer than the terminal (default 25 lines; UG p. 49) halts waiting for Enter
  (line) or space (page); `set cli terminallines <disable|N>` changes it. A non-interactive
  driver must feed spaces or set terminallines once per session. Prompt is
  `[<system-name>]> ` (`[serserv]> ` on the lab unit).
- Unique-prefix abbreviation is accepted (`se dev po 1 te en`), parameter values must be typed
  in full; string params clear with literal `CLEAR` (all caps), IP params with `0.0.0.0`
  (UG p. 49, p. 273).
- `show deviceport port <N> display ip|data|modem|logging|device` filters output to one section
  — use `display ip` for the Connection Settings resource; it's line-oriented
  (`^Label: value$`) and unambiguous, unlike the three-column full view.

## 6. Surprises / gotchas

1. **No RFC 2217.** Zero mentions in the 351-page UG or the firmware image, and no such service
   in device help. Not surprising: RFC 2217 is Jan 2008; SLC 5.2a shipped Aug 2007. The raw TCP
   In listener (plus Lantronix "Com Port Redirector" on the client, UG p. 159) is the
   5.2a-era answer to virtual serial ports. Don't model `rfc2217` per-port enablement.
2. **`maxdirect` is global on 5.2a**, not per-port: device help lists it under
   `set deviceport global`, `show deviceport global` prints one value for all ports, and the
   internal config model has `GLOBAL/MAX_DIRECT` only. The UG Rev F appendix lists it among
   per-port params — UG drift (the guide is Rev F, April 2008; the device runs Aug-2007-era
   5.2a). Trust the on-device `help` synopsis for 5.2a.
3. **UG-vs-5.2a parameter drift, both directions.** Not in the 5.2a help synopsis but present
   in UG Rev F: `sshtimeout`/`tcptimeout`/`telnettimeout`, `showlines`, `viewportlog`,
   `portlogseq`, `group`, `dialbackretries`, `cbcptype`/`cbcpnocallback`, `usblogging*`,
   extended `modemstate` values (`dialinhostlist|cbcpserver|cbcpclient`), and `device` values
   beyond `none|slp8|slp16`. Conversely the UG's per-port list omits nothing that 5.2a has.
   Per-firmware-version capability probing (parse `help set deviceport`) is the robust move.
4. **No "raw vs cook" vocabulary.** The cooked/raw distinction is encoded in *which service*
   you enable (Telnet In/SSH In = login + banner + break-sequence handling; TCP In = raw
   bridge), plus `tcpauth` and the "clear `breakseq` for binary" convention. Model it as three
   services, not a mode enum.
5. **Enablement and port number are independent.** `show deviceport port` prints listener
   numbers even for disabled services, and per-port overrides persist across enable/disable.
6. **Auth defaults are asymmetric**: Telnet/SSH-In auth default *on*, TCP-In auth default *off*
   (factory XML + UG). A provider that defaults all three the same way will surprise users.
7. **Per-port dedicated IP** (`ipaddr`) is a second, easily missed half of "how a port is
   reached": standard 23/22 on a per-serial-console IP address. Bound to Eth1 only; conflicts
   with Ethernet bonding (UG p. 97).
8. The lab unit deviates from factory defaults in ways that look like the UG lying: Telnet In
   enabled + Telnet Auth disabled on ports, global starting ports moved to 2000/3000/4000
   (factory XML: 2001/3001/4001), maxdirect=2. Always compare against `xml-defaults`, not the
   live unit, when reasoning about defaults.
9. `show connections` mixes user sessions and configured port↔endpoint connections in one
   runtime table with volatile IDs — fine for data sources, wrong as a config source of truth.

## Sources

- `vendor_firmware/SLC_UG.pdf` (SLC Console Manager User Guide, Rev F Apr 2008):
  ch. 5 CLI conventions pp. 48–50; ch. 7 Services pp. 68–75 (`set services`/`show services`
  p. 74); ch. 8 Device Ports pp. 89–105 (connection methods pp. 89–90, port numbering p. 93,
  per-port settings pp. 94–98, CLI commands pp. 104–105); ch. 11 Connections pp. 157–164;
  ch. 15 Command Reference pp. 270–273; Revision History p. ii.
- `vendor_firmware/ext/slcupdate-5.2a/www/help/Device_Port_Settings.htm`,
  `www/help/port_access.htm`, `www/help/Services.htm`, `www/help/connections.htm` (web help as
  shipped in firmware 5.2a).
- `vendor_firmware/ext/slcupdate-5.2a/www/xml-defaults/devports.dtd` and `devports.xml`
  (internal config model and factory defaults: `GLOBAL/MAX_DIRECT`,
  `GENERAL/TELNET_*|SSH_*|TCP_*|TELSSH_IPADDR|WEBCOLS|WEBROWS`).
- Live SLC48, firmware 5.2a, read-only Telnet session at 192.168.0.197 (Nov 2026): `help`,
  `help set deviceport`, `show services`, `show deviceport global`, `show deviceport names`,
  `show deviceport port 1-2`, `show deviceport port 1 display ip`,
  `show portstatus deviceport 1-3`, `show connections`. No configuration was changed; the
  administrator password was used only to log in.
