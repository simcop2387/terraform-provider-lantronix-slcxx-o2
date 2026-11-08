# Research: SLC CLI surface for Device Port settings (serial params)

Resolves issue #2. Question (verbatim):

> exact CLI command surface for Device Port settings on a Lantronix SLC48 running firmware 5.2a: commands to SHOW and SET per-port serial parameters (baud rate, data bits, parity, stop bits, flow control, plus related params like XON/XOFF chars, local echo); whether settings are strictly per-port or involve profiles/global defaults; exact verbatim output format of the show-side commands (for parser design).

Evidence: Lantronix SecureLinx Console Manager User Guide (`vendor_firmware/SLC_UG.pdf`, extracted with `pdftotext -layout`), on-device web help (`vendor_firmware/ext/slcupdate-5.2a/www/help/Device_Port_Settings.htm`), config-file DTD (`vendor_firmware/ext/slcupdate-5.2a/www/xml-defaults/devports.dtd`), and a READ-ONLY telnet session against the live SLC48 (model banner confirmed; firmware 5.2a per ticket). All transcripts below are verbatim from the live device except that CR/NUL control bytes were stripped.

## Answer summary

### Serial parameters: strictly per-port

Each of the 48 device ports carries its own serial settings block. There are **no profiles and no global serial defaults**. Confirmed three ways:

1. Storage schema: `devports.dtd` — `<!ELEMENT DEVPORT (PORTID,NAME,GENERAL,SERIAL,MODEM,LOGGING)>` with `<!ELEMENT SERIAL (BAUD,DATABITS,STOPBITS,PARITY,FLOWCTRL,LOGINS,CONN_DSR,DISCONN_DSR)>`. The sibling `GLOBAL` element contains only `START_TELNET_PORT`, `START_SSH_PORT`, `START_TCP_PORT`, `MAX_DIRECT` — no serial fields.
2. `show deviceport global` on the live device returns only starting TCP port numbers and a max-direct-connects limit (transcript below).
3. `help set deviceport` on the live device shows `set deviceport global` accepting only `telnetport|sshport|tcpport|maxdirect`.

Bulk configuration is done by passing a **port list** to the per-port command (`2-5,6,12,15-16`, comma-separated, ranges with `-`; the User Guide also documents `ALL`), not by a profile object.

### SET side (write semantics owned by another ticket; surface only)

```
set deviceport port <Device Port # or List or Name> <one or more parameters>
```

Serial-relevant parameters (from live-device `help set deviceport`, firmware 5.2a):

| Parameter | Values |
|---|---|
| `baud` | `<300-115200>` (device help; see surprise #3) |
| `databits` | `<7|8>` |
| `parity` | `<none|odd|even>` |
| `stopbits` | `<1|2>` |
| `flowcontrol` | `<none|xon/xoff|rts/cts>` |
| `logins` | `<enable|disable>` (login prompt on the serial device) |
| `checkdsr` / `closedsr` | `<enable|disable>` (hardware signal triggers) |
| `breakseq` | `<1-10 Chars>` (e.g. `\x1bB`; accepts `CLEAR`) |

Success response for a set (User Guide ch. 14 example, p. 246):

```
[SLC]> set deviceport port 2 baud 57600 flowcontrol none
Device Port settings successfully updated.
```

**XON/XOFF characters and local echo are not configurable on this platform.** The full 5.2a parameter list (device help + User Guide command reference, ch. 15 pp. 270–273) contains no xon/xoff character parameters and no echo parameter; `flowcontrol xon/xoff` uses fixed standard characters. There is nothing to model for those in a provider schema.

The management **console port** is a separate, single object: `set consoleport <params>` / `show consoleport`, with the same serial params (`baud`, `databits`, `stopbits`, `parity`, `flowcontrol`) plus `group`, `showlines`, `timeout` (User Guide pp. 114–115).

### SHOW side

From live-device `help set deviceport` synopsis (matches User Guide ch. 15):

```
show deviceport port <Device Port List or Name> [display <ip|data|modem|logging|device>]
show deviceport names
show deviceport global
show portstatus [deviceport <Device Port List or Name>] [email <Email Address>]
show portcounters [deviceport <Device Port List or Name>] [email <Email Address>]
show consoleport
```

`display data` is the parser-friendly mode: one field per line, single column.

## Verbatim live transcripts (SLC48, firmware 5.2a)

### `help set deviceport` (truncated to serial-relevant portion; full synopsis on device)

```
[serserv]> help set deviceport
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
...
baud <300-115200>                   databits <7|8>
stopbits <1|2>                      parity <none|odd|even>
flowcontrol <none|xon/xoff|rts/cts> logins <enable|disable>
checkdsr <enable|disable>           closedsr <enable|disable>
...
Note: A group of device ports can be configured by specifying a
      comma-separated list of ports (i.e., '1-4,8,10-12').
...
show deviceport global
show deviceport names
show deviceport port <Device Port List or Name>
                [display <ip|data|modem|logging|device>]
```

### `show deviceport global`

```
[serserv]> show deviceport global
___Current Device Port Global Settings_________________________________________
Starting Telnet Port: 2000
Starting SSH Port: 3000
Starting TCP Port: 4000
Maximum # of Direct Connects per Device Port: 2
```

### `show deviceport port 1` (full, default display)

Note the three interleaved fixed-width columns (Modem | Data | IP), then Logging and Device sections:

```
[serserv]> show deviceport port 1
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

### `show deviceport port 2 display data` (section-filtered — recommended parse target)

```
[serserv]> show deviceport port 2 display data
___Current Device Port Settings________________________________________________
Number: 2  Name: Port-2
Banner: <none>

Data Settings------------------------------------------------------------------
Baud Rate: 115200
Data Bits: 8
Stop Bits: 1
Parity: none
Flow Control: none
Logins: disabled
Check DSR: disabled
Close DSR: disabled
Break Seq: \x1bB
```

(Values are space-padded to a fixed width — e.g. `Baud Rate: 9600  `, `Flow Control: none    ` — so trim trailing whitespace.)

### Multi-port list: one block per port, header repeated

```
[serserv]> show deviceport port 3-4 display data
___Current Device Port Settings________________________________________________
Number: 3  Name: Port-3
Banner: <none>

Data Settings------------------------------------------------------------------
Baud Rate: 9600
Data Bits: 8
Stop Bits: 1
Parity: none
Flow Control: none
Logins: disabled
Check DSR: disabled
Close DSR: disabled
Break Seq: \x1bB
___Current Device Port Settings________________________________________________
Number: 4  Name: Port-4
Banner: <none>

Data Settings------------------------------------------------------------------
Baud Rate: 9600
Data Bits: 8
Stop Bits: 1
Parity: none
Flow Control: none
Logins: disabled
Check DSR: disabled
Close DSR: disabled
Break Seq: \x1bB
```

### `show deviceport names` (two-column grid, zero-padded numbers)

```
[serserv]> show deviceport names
___Current Device Port Names___________________________________________________
 01 - Port-1                         25 - Port-25
 02 - Port-2                         26 - Port-26
 ...
 24 - Port-24                        48 - Port-48
```

### `show portstatus deviceport 1` / `show portcounters deviceport 1` (runtime state, not config)

```
[serserv]> show portstatus deviceport 1
Device Port:                     1      DSR/CD:                        Yes
Name:                       Port-1      DTR:                           Yes
Mode:                         Idle      CTS:                           Yes
                                        RTS:                           Yes

[serserv]> show portcounters deviceport 1
Device Port:                     1      Seconds since zeroed:         5764
Bytes input:                     0      Bytes output:                    0
Framing errors:                  0      Flow control errors:             0
Overrun errors:                  0      Parity errors:                   0
```

### `show consoleport` (management console port, separate object)

```
[serserv]> show consoleport
___Current Console Port Settings_______________________________________________
Baud Rate: 115200
Data Bits: 8
Stop Bits: 1
Parity: none
Flow Control: none
Timeout: disabled
```

## Parser design notes

- Prompt is `[<hostname>]> ` (live unit: `[serserv]> `); responses end with the prompt, no pager hit for these shows.
- Prefer `show deviceport port <n> display data` — one `Label: value` per line. The default (unfiltered) display interleaves three fixed-width columns and is fragile to parse; field positions vary with which rows exist.
- Section banners are `___Current Device Port Settings____...` / `Data Settings----...`; a multi-port query repeats the full block per port — split on the `Number: <n>  Name:` line (note the **two** spaces between number and `Name:`).
- Empty values render as `<none>`; enable/disable rendered lowercase; parity `none|odd|even`; flow control label is `Flow Control` with the value space-padded.
- Live 5.2a labels differ from the User Guide's example transcript (p. 245): live adds `Telnet Auth`, `SSH Auth`, `TCP Auth`, `DOD *`, `Caller ID *`, `Web Columns/Rows`, and a `Device Settings` section; the guide's sample lacks them. **Target the live format.**
- The serial param labels to match: `Baud Rate:`, `Data Bits:`, `Stop Bits:`, `Parity:`, `Flow Control:`.

## Open surprises / discrepancies

1. **No XON/XOFF-char or local-echo settings exist** on SLC48 5.2a (device help, User Guide ch. 15, and `devports.dtd` all agree). The ticket's assumption here does not hold; skip these in the schema.
2. `?` is not a help trigger at the top level ("Unknown parameter: '?'"), and the documented `<command> ?` form also fails for `set deviceport port 1 ?`. Use `help set deviceport` / `help <command>` instead.
3. Baud range disagreement: live-device help says `baud <300-115200>`; User Guide command reference (p. 270) says `<300-230400>`; and ch. 2 prose says "eight baud-rate options" then lists ten (300, 600, 1200, 2400, 4800, 9600, 19200, 38400, 57600, 115200). Trust the device: treat the valid set as the ten discrete values up to 115200.
4. `set deviceport global` on the device accepts `maxdirect`, which the User Guide ch. 8 "Global Commands" list omits (it does appear in the ch. 15 per-port parameter list). Another doc/codegen gotcha: trust device help.
5. `exit` is rejected at the prompt ("Unknown parameter: 'exit'") — logout command unknown; sessions terminate on TCP close. Untested further to stay read-only-adjacent.
6. The live unit's ports 1–2 are set to 115200/Telnet-enabled while 3–4 sit at 9600 defaults — confirms independent per-port state, but means "defaults" (baud 9600, 8N1, flow none) should be verified against a factory-fresh unit before being baked into the provider.

## Sources

- `vendor_firmware/SLC_UG.pdf` — ch. 8 "Device Ports" (pp. 92–105, web-page fields + CLI equivalents incl. full `set deviceport port` parameter list), ch. 14 example with verbatim `show deviceport port 2` / `set ... baud/flowcontrol` transcript (pp. 245–246), ch. 15 "Command Reference — Device Port Commands" (pp. 270–274, incl. `display <ip|data|modem|logging|device>` option and CLEAR-able params).
- `vendor_firmware/ext/slcupdate-5.2a/www/help/Device_Port_Settings.htm` — on-device help; matches the User Guide (Data Settings: Baud, Data Bits, Stop Bits, Parity, Flow Control defaults 9600/8/1/none/none).
- `vendor_firmware/ext/slcupdate-5.2a/www/xml-defaults/devports.dtd` — per-port `SERIAL` element; `GLOBAL` has no serial fields.
- Live telnet session, SLC48 (hostname `serserv`), read-only commands only: `help`, `help set deviceport`, `show deviceport global|names|port <n> [display data]`, `show portstatus`, `show portcounters`, `show consoleport`. Credentials from `secrets.txt`, never recorded.
