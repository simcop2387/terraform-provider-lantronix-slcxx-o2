# Research: config persistence and apply semantics (set → run → save)

**Issue:** #4 · **Device:** Lantronix SLC48, firmware 5.2a (live unit `serserv`, 192.168.0.197) · **Date:** 2026-11-08
**Sources:** `vendor_firmware/SLC_UG.pdf` (pdftotext extraction), help trees under `vendor_firmware/ext/slcupdate-5.2a/www/help*/`, and live CLI sessions over telnet (read + one reverted `set`).

## Question

Persistence and apply semantics of SLC CLI configuration changes on firmware 5.2a:

1. Do `set` commands take effect immediately in the running config, or need a commit/save?
2. Is `admin config save` required for changes to survive reboot, and what exactly does it persist?
3. Do Device Port / Connection Settings changes require device reboot, or only port/service restart?
4. Can the full configuration be dumped machine-readably in one shot, or only via per-section `show` commands?

## Answer (summary)

1. **Immediate.** The SLC has no running/startup distinction and no commit command. A `set` writes the single live configuration store, takes effect immediately, and is persisted automatically. There is nothing to "commit".
2. **No — `admin config save` is not a commit step.** It exports a *named snapshot file* to local/FTP/SFTP/NFS/CIFS/PC-card/USB storage for backup, restore, or dual-bank copy. Changes survive reboot without it. What the snapshot persists: all configuration parameters, optionally plus SSH keys, SSL cert and scripts (`savesshkeys|savesslcert|savescripts`).
3. **No reboot for Device Port or Connection Settings.** Those chapters carry no "must reboot" note anywhere; changes apply immediately (verified live). A reboot is documented as required only for a specific list of daemon-level options (see below). Restoring a saved configuration *does* auto-reboot the device.
4. **Yes: `show sysconfig`** reports *all* configurable parameters in one command (`display <basic|auth|devices>` narrows it). Output is fixed-column human-readable text (secrets masked), not XML/JSON. The firmware's internal web XML interface is per-section only — there is no single machine-format config document; saved snapshot files are opaque restore blobs.

## 1. `set` takes effect immediately; there is no commit/save step

The live command tree contains no commit/write-memory style command at all — the only state-touching verbs are `set`, `connect`, `diag`, and `admin {reboot|shutdown|ftp|config|firmware|...}`:

```text
[serserv]> help
set       network|ipfilter|routing|datetime|ntp|services|nfs|cifs|menu|history|
          auth|localusers|remoteusers|nis|ldap|radius|kerberos|tacacs+|sshkey|
          consoleport|deviceport|locallog|command|slcnetwork|cli|password
show      network|ipfilter|routing|datetime|ntp|services|nfs|cifs|menu|history|
          auth|localusers|remoteusers|nis|ldap|radius|kerberos|tacacs+|sshkey|
          consoleport|deviceport|locallog|connections|pccard|portstatus|
          portcounters|syslog|auditlog|sysstatus|sysconfig|slcnetwork|cli|user
connect   direct|listen|bidirection|unidirection|terminate
diag      ping|loopback|traceroute|arp|lookup|netstat|perfstat|sendpacket|
          nettrace|internals
pccard    storage|modem
admin     reboot|shutdown|ftp|config|firmware|version|keypad|quicksetup|banner|
          web|events|lcd
logout    terminates CLI session
```

(The UG's full command table additionally lists `admin clear|memory|site` and category `usb1` on USB models — still no commit/write-memory command anywhere.)

The user guide states immediacy in several places. Quick Setup (front-panel keypad), Ch. 4, p. 35:

> Any changes made to the network, console port, and date/time settings take effect immediately.

Web UI glossary, Ch. 5 ("Apply Button") — and every CLI section in the UG is explicitly introduced as *"The following CLI commands correspond to the web page entries described above"*, i.e. `set <x>` is exactly the Apply button for the same underlying setting:

> **Apply Button:** Apply on each web page makes the changes immediately and saves them so they will be there when the SLC console manager is rebooted.

That sentence is the crux: apply *and* persist are one operation; there is no separate save that survives reboot. The audit log corroborates the single-store model — it "save[s] a history of all configuration changes" with no notion of uncommitted state (Ch. 7, Services page).

Live confirmation (§ transcripts): `set deviceport port 48 baud 19200` returned `Device Port settings successfully updated.` and the very next `show deviceport port 48` reflected 19200 — no intermediate "pending" state exists to promote.

## 2. `admin config save` = snapshot export, not persistence

Syntax (Command Reference, Ch. 15):

```text
admin config save <Config Name> location <local | ftp | sftp | nfs | cifs | pccard | usb>
    [nfsdir <NFS Mounted Directory>] [usbport <U1>] [pccardslot <upper|lower>]
    [savesshkeys <enable|disable>] [savesslcert <enable|disable>] [savescripts <enable|disable>]
```

Description: "Saves the current SLC configuration to a selected location." The surrounding commands make the intent unambiguous — this is snapshot *file management*: `admin config restore` (returns the unit to a named snapshot; "A reboot will automatically be done after a successful config restore or factory defaults"), `admin config rename|delete`, `admin config copy <current|Config Name>` (copy to the other bank on dual-boot units), and `admin config show <location>` (list snapshots).

Scope of a snapshot: all configuration parameters — the `preserveconfig` parameter for restore/factorydefaults enumerates the full parameter space (`nt` Networking, `sv` Services, `dt` Date/Time, `dp` Device Ports, `ra` Remote Authentication, `lu` Local Users, `ub` USB / `pc` PC Card) — plus optionally SSH host keys, the SSL certificate and CLI scripts via the three `saves*` flags.

Evidence that persistence across reboot does **not** depend on it: (a) the Apply-button sentence above; (b) live `admin config show default` lists exactly one snapshot, a 2007 factory image, while the unit has clearly been reconfigured and rebooted many times since without any new save:

```text
[serserv]> admin config show default
___Current Saved Configurations________________________________________________
Location: Default
Configuration: 05-24-07            Saved: 05/23/07 12:26:58
```

## 3. Apply requirements for Device Ports / Connections vs. reboot-only settings

The Device Ports (Ch. 8) and Connections (Ch. 11) material contains **no** reboot/restart requirement — the only "Restart Delay" fields there are connection-reestablishment timers, not apply mechanics. Changing a device port's serial or access parameters applies to the live configuration immediately (verified: baud change visible in `show` seconds later). Connections: editing an *active* connection is done by `connect terminate` / Terminate, then re-creating it (Ch. 11 "To view, update, or disconnect a current connection"); the trigger type governs post-reboot re-establishment ("Immediately. These connections are always re-established after reboot.") — that is boot behavior, not an apply requirement.

By contrast, the UG explicitly requires a **reboot** for these (and only these) settings:

- Services page: SSH login Timeout, SSH Port, SSH V1 Logins; Telnet login Timeout ("You must reboot the unit before a change will take effect.")
- Web server page: Allow SSLv2 Protocol, Cipher selection ("Changing this option requires a reboot for the change to take effect.")
- Every remote-authentication method: NIS, LDAP, RADIUS, Kerberos, TACACS+ ("Note: You must reboot the unit before your changes will take effect.")
- Firmware update and config restore/factorydefaults (restore reboots automatically)

Implication for the provider: after any `set`, no apply action is needed; only the listed daemon-level options need an explicit `admin reboot` to activate, and the provider should surface that as a plan-time warning rather than performing reboots.

## 4. One-shot configuration dump

**`show sysconfig [display <basic|auth|devices>] [email <addr>]`** — Ch. 15: *"To display a snapshot of configurable parameters… Displays a report of all configurable parameters or a shorter report with basic system settings, authentication settings, or device settings."* Verified live: `show sysconfig display basic` returned every network/routing/datetime/services/auth/console/server section in one response (secrets masked as `****`); `show sysconfig display devices` returned the per-port blocks. With `set cli terminallines disable` the output streams unpaged, so a full unfiltered `show sysconfig` is a single-shot complete dump.

Format caveats for machine parsing: fixed-width two/three-column text (`Label: value` pairs at known column offsets), section headers underlined with dashes, `[none]` for unset values — parseable but layout-based, not delimited. There is **no** XML/JSON one-shot export: the firmware's internal web interface uses per-section DTDs (`www/xml/devports.dtd`, `network.dtd`, `connections.dtd`, … in slcupdate-5.2a) with no whole-device document, and that interface is undocumented in the UG. Saved snapshot files (`admin config save`) are restore blobs — format undocumented, not a readable export. Per-section `show <category>` commands remain the most structured source; `show sysconfig` is the one-shot overview.

## Live verification transcript (set → verify → restore)

Telnet to 192.168.0.197:23 as "SLC User" (credentials redacted; never logged). Guardrails honored: Telnet service config, network configuration and sysadmin password untouched; no reboot issued. Only device port 48 (unused, default-named `Port-48`) and the per-user CLI paging setting were modified, both reverted.

```text
> login: <REDACTED>
Password: <REDACTED>

Welcome to the SecureLinx Console Manager
Model Number: SLC48
For a list of commands, type 'help'.

[serserv]> set cli terminallines disable        # session paging off (restore at end)
CLI settings successfully updated.

[serserv]> show deviceport port 48              # PRIOR STATE
___Current Device Port Settings________________________________________________
Number: 48  Name: Port-48
...
Modem State: disabled               Baud Rate: 9600        Telnet: enabled
Modem Mode: Text                    Data Bits: 8           Telnet Port: 2048
...(full block captured; telnet port 2048, SSH disabled, TCP disabled)...

[serserv]> set deviceport port 48 baud 19200    # CHANGE
Device Port settings successfully updated.

[serserv]> show deviceport port 48              # VERIFY — immediate effect
...
Modem State: disabled               Baud Rate: 19200       Telnet: enabled
...

[serserv]> set deviceport port 48 baud 9600     # RESTORE
Device Port settings successfully updated.

[serserv]> show deviceport port 48              # VERIFY RESTORED — byte-identical to prior state
...
Modem State: disabled               Baud Rate: 9600        Telnet: enabled
Modem Mode: Text                    Data Bits: 8           Telnet Port: 2048
...(all remaining fields identical to prior dump)...

[serserv]> set cli terminallines 25             # restore documented default
CLI settings successfully updated.
[serserv]> show cli
___Current CLI Settings _______________________________________________________
SCS Commands: disabled
Number of terminal lines: 25
[serserv]> logout
Logging out...
```

Note: `set cli terminallines` is retained per-user between sessions, so it was restored to the documented default (25). Full unredacted-credential-free transcripts: `/tmp/opencode/persist-research/session.log`, `session2.log` (throwaway host).

## Implications for the provider

- Model each setting as direct apply: Read = `show <section>`, Create/Update = `set ...`, no apply/commit resource, no pending-state handling.
- Never call `admin config save/restore` in normal operation — they are user-facing backup tools (and restore reboots the device).
- Flag the reboot-required settings (SSH/Telnet timeouts & port, SSHv1, web SSLv2/cipher, remote-auth methods) as requiring manual `admin reboot`.
- Prefer per-section `show` parsing over `show sysconfig` for drift detection; use `show sysconfig` for a one-shot overview/diagnostic dump.
