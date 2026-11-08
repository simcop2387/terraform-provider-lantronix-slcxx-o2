# Research: telnet session mechanics for automation (login, prompts, paging)

Status: research findings, feeds Transport contract ticket #6.
Device under test: Lantronix SLC48, firmware 5.2a (`admin version` confirmed), telnet at `192.168.0.197:23`, local user `sysadmin` (Administrators group).

## Question (from issue #5)

What must a robust Go telnet client handle to script the SLC48 (firmware 5.2a) CLI reliably: login handshake quirks (`transcript.txt` shows a leading `Logging out...` banner and a failed-first-login pattern — is there stale-session or double-prompt behavior?); password echo behavior; prompt formats and how they vary by mode/user; paging — does long output pause for space/return, how is the "more" state detected and driven; telnet protocol negotiation observed (line vs character mode, ECHO/NAWS); idle timeout/auto-logout; logout command and clean disconnect. Byte-level quirks (CR/LF handling, trailing prompts).

## Method

- **Raw Python socket probes** (`/tmp/opencode/probe*.py`, logs `probe*.log`) recording every received byte with `repr()`, full IAC parsing (all options answered `DONT`/`WONT` unless stated). This was the primary instrument; the telnet MCP tools were deliberately not used for credentialed sessions so passwords never pass through a transcripted tool call. ~10 login/logout cycles, 0 failures when prompts were matched exactly.
- **Firmware strings**: `vendor_firmware/ext/slcupdate-5.2a/bin/cli` (the CLI binary) and `bin/x15login` (the login/telnet-daemon binary) — exact prompt/pager/timeout message formats.
- **Docs**: `vendor_firmware/SLC_UG.pdf` (pdftotext), `www/help/Services.htm`.
- Read-only commands only (`help`, `show`, `admin version`). No service/network/password config was touched. Credential values are not reproduced anywhere in this file.

## TL;DR for the Transport contract (#6)

1. Line-mode server that **echoes input itself** (even when ECHO negotiation is refused); client must not local-echo, and passwords are never echoed.
2. Wait for exact byte prompts: `login: `, `Password: `, then CLI prompt `[<hostname>]> ` (here `[serserv]> `, trailing space, no trailing newline). Auth latency after password is ~0.5–1 s.
3. No paging on this device (`set cli terminallines` = disabled). The pager exists in firmware: prompt string ` -- Press <space> for next page, <enter> for next line, or 'q' to quit --`; drive with space (page) / CR (line) / q. Detect defensively with a regex; leave driving dormant unless a more-prompt is seen.
4. Server pushes option proposals twice (`DO TTYPE/TSPEED/XDISPLOC/NEW-ENVIRON`, `WILL SGA`, `DO ECHO`, `DO NAWS`, `WILL STATUS`, `DO TOGGLE-FLOWCONTROL`, then re-proposes `WILL SGA`/`WILL ECHO` ~200 ms later). Answer every WILL/DO idempotently; nothing has to be accepted.
5. Idle timeout: `timeouttelnet <disable|1-30 minutes>`, currently **disabled** (session survived 120 s idle; message when it fires is `Idle timeout. Logging out...`).
6. `logout` → echo + `\r\nLogging out...\r\n\r\n`, then the server closes TCP within ~50 ms. The leading `Logging out...` in `transcript.txt` is this **session-end banner captured at the tail of a previous connection**, not a connect-time artifact — fresh connections always start with a clean banner+`login:` (verified incl. after abrupt RST).
7. Byte quirks: pre-login banner uses bare LFs and ends `...\r\x00login: ` (CR + NUL!); post-login output is CRLF; CR, LF or CRLF all terminate commands; echo of the command line precedes every response; TCP chunk boundaries are arbitrary (a prompt can be split mid-string).

## 1. Login handshake

Raw byte transcript (client refused all telnet options; `<user>`/`<pass>` redacted):

```
connect → server IAC proposals (see §4)
RECV b'\r\n\n\r\n\nWelcome to the SLC\n\n\n\n'
RECV b'\r\x00login: '                          # CR + NUL before "login:"
SEND <user> CRLF
RECV b'sysadmin\r\nPassword: '                 # server echoes username, then password prompt
SEND <pass> CRLF
RECV b'\n'                                     # terminates the Password line; NO password chars echoed
RECV b'\r\n'                                   # ~50 ms later
RECV b'\r\nWelcome to the SecureLinx Console Manager\r\nModel Number: SLC48\r\nFor a list of commands, type \'help\'.\r\n\r\n[serserv]> '
                                               # ^ arrives ~0.5–1.0 s after password CRLF (auth latency)
```

Findings:

- **Password echo**: none. Only the post-password `\n`/`\r\n`; suppression is entirely server-side, so a client that also echoes locally would double-print the username but never the password.
- **Early input / race**: sending `user CRLF pass CRLF` immediately at connect (before any bytes were received) still logs in — input is buffered by `x15login` and processed in order. So a failed first login cannot be explained by lost keystrokes on a *fresh* connection.
- **Stale-session gating: none.** Every new TCP connection gets the clean banner + `login:` — including immediately (0.5 s) after a previous logged-in session was killed with a TCP RST (`SO_LINGER 0`), and after 30 s. No double prompt, no "session in use" state.
- **Concurrent sessions**: two simultaneous `sysadmin` CLI logins coexist; neither is kicked (verified via `show connections`, ids 52/53 both `sysadmin`). Scripts do not fight over the CLI. Note `show connections` also lists connected-but-unauthenticated telnet slots with blank User column — half-open slots linger, so always `logout` + close rather than dropping sockets.
- **`transcript.txt` interpretation**: `bin/cli` contains the string `Logging out...` printed on logout (observed live: `logout\r\nLogging out...\r\n\r\n` then FIN). The transcript's line 1 is therefore the *end of a previous session* in one continuous capture, followed by a reconnect (`Welcome to the SLC`, `login:`). The failed first login after it is consistent with credentials being typed/piped while the client was still in the old session or mid-reconnect (the device also logs auth failures via PAM: syslog shows `pam_unix(login:auth): check pass; user unknown`). It did not reproduce across ~10 properly-paced attempts.
- **Lockout hazard**: `x15login` contains `Too many incorrect login attempts.` — repeated bad logins are punished (threshold not probed, to avoid locking the admin account). A robust client should retry at most once after an `Incorrect Login or Username` and then fail loudly.

## 2. Prompt formats

- CLI prompt is built from format string `"[%s]> "` (`strings bin/cli`) where `%s` = **hostname** — confirmed: `show network host` → `Hostname: serserv`; UG: "The hostname will be shown in the CLI prompt." Default hostname is `slcXXXX` (last 4 of serial), so do not hardcode `serserv`.
- The prompt has a **trailing space and no trailing newline**; it is re-emitted after every command and after every stray CR (an empty line yields just `\r\n[serserv]> ` — this is the "double prompt" seen in `transcript.txt`, normal behavior, not an error).
- Menu-mode users (menu interface instead of full CLI) get prompts shaped like `[Enter 1-%d]> ` (`bin/cli` format string; UG ch.13 example `[Enter 1-4]>`). Full-CLI users such as `sysadmin` (Administrators group) get the plain `[host]> `.
- After output, framing is deterministic: `<echo of command>\r\n<payload>\r\n[host]> `. Errors do not add blank lines: `show\r\nIncomplete Command.\r\n[serserv]> `.
- Suggested readiness regex over IAC-stripped text: `\[[^\]\r\n]+\]> \z` at end of buffer (anchored, trailing space significant).

## 3. Paging

- **This device never pages.** `show portstatus` (14,718 bytes / ~380 lines), `show syslog` (8 KB) and `show auditlog` (32 KB) streamed continuously in ~12 s with no pause, no flow-control IAC. Reason: `show cli` → `Number of terminal lines: disabled`.
- Accepting NAWS and advertising an 80×10 window changed nothing — the `terminallines` setting overrides auto-detection.
- Mechanism (docs + firmware strings): output longer than N lines halts; **Enter = one more line, space = one more page, `q` = quit**; pager string emitted by `bin/cli` is exactly:

  ```
   -- Press <space> for next page, <enter> for next line, or 'q' to quit --
  ```

  Controlled by `set cli terminallines <disable|1-120>`; UG (ch.5, p.49–50) says default window is 25 lines and the feature auto-detects terminal size "if the SLC console manager cannot detect the size of the terminal automatically" (i.e., via NAWS when enabled).
- Transport guidance: do not enable paging on the device for tests (config change); instead match `-- Press <space> for next page` (or a looser `(?i)press <space>|for next page`) at buffer tail after each chunk; if seen, drive space/CR/q and keep draining. Treat absence as normal for firmware 5.2a with the current config.

## 4. Telnet protocol negotiation

Observed at connect (every connection, same pattern):

```
S: DO TTYPE(24) DO TSPEED(32) DO XDISPLOC(35) DO NEW-ENVIRON(39)
S: WILL SGA(3)  DO ECHO(1)  DO NAWS(31)  WILL STATUS(5)  DO TOGGLE-FLOWCONTROL(33)
   (~200 ms later, after client refused everything:)
S: WILL SGA(3)  WILL ECHO(1)
```

- The server proposes **both `DO ECHO` and `WILL ECHO`** (contradictory) and re-proposes after refusals — clients must answer every proposal idempotently and never treat a repeat as an error.
- With *all* options refused the session still works: it is classic **line mode with server-side line editing and server echo** — typed input arrives echoed in one chunk at EOL (e.g. `show portstatus\r\n`), there is no per-character traffic, and no `GA`/`SUPDUP`. Accepting NAWS is optional (no observable effect here). No `SB` subnegotiation was ever sent by the server (it only asks).
- Client obligations: parse and strip IAC everywhere (including escaped `IAC IAC` and `SB...SE`, which can split across TCP segments), reply to every WILL/DO, tolerate unknown options (35/39/33 arrived unannounced by any standard client-side default list).

## 5. Idle timeout / auto-logout

- CLI: `set services timeouttelnet <disable|1-30 minutes>` (`help set services`); device state `show services` → `Telnet Timeout: disabled`. A logged-in session idled 120 s with zero traffic: no keepalive, no warning, no disconnect — consistent with "disabled".
- When enabled, the auto-logout banner is `Idle timeout. Logging out...` (`bin/cli`) followed by connection close — distinct from the manual `Logging out...`. Console ports and SSH have separate timeouts (console 1–30 min per UG; `timeoutssh` same range).
- Transport guidance: don't rely on idle disconnects to clean up; send keepalive-free but short-lived sessions, and always logout.

## 6. Logout & clean disconnect

```
SEND b'logout\r'
RECV b'logout\r\nLogging out...\r\n\r\n'      # echo + banner, <10 ms
TCP FIN from server ~30–60 ms later
```

- `logout` is the only command that ends a session (`help`: "logout terminates CLI session"); after FIN the slot is gone. Closing TCP without logout also works but leaves unauthenticated/ghost entries in `show connections` until the stack notices, so scripted clients should always `logout`, read the banner (or just FIN), then close.
- A second login on the same account does **not** force the first session out, so there is no takeover race to handle.

## 7. Byte-level quirks summary

| Aspect | Observation |
|---|---|
| Pre-login line endings | bare `\n` (`\r\n\n\r\n\nWelcome to the SLC\n\n\n\n`) |
| Before `login:` | `b'\r\x00'` — CR followed by NUL byte; strip NULs |
| Post-login line endings | consistently `\r\n` |
| Command terminator | `\r`, `\n` or `\r\n` all accepted (tested); server normalizes echo to `<cmd>\r\n` |
| Prompt | `[<hostname>]> ` — trailing space, no newline; repeated on empty input |
| Echo | server echoes everything except password, even when ECHO refused |
| Chunking | arbitrary; prompts/responses split across reads (e.g. `\r\x00login: ` arrived alone after banner chunk) — buffer until prompt regex matches |
| Throughput | ~1.2 KB/s on big outputs (`show portstatus`), no flow control requested |

## 8. Sources

- Live probes against 192.168.0.197:23, 2026-11-08 (raw-socket logs `probe.log`/`probe2.log`/`probe3.log`/`probe4*.log`/`probe5.log`/`probe6.log` in the research sandbox; key transcripts quoted above).
- `vendor_firmware/ext/slcupdate-5.2a/bin/cli` strings: `"[%s]> "`, `"Enter 1-%d]> "`, pager string, `Idle timeout. Logging out...`, `Logging out...`, `Invalid terminallines: %s (must be 'disable', or between 1 and 120).`
- `vendor_firmware/ext/slcupdate-5.2a/bin/x15login` strings: `Welcome to the SLC`, `login: `, `Incorrect Login or Username`, `Too many incorrect login attempts.`
- `vendor_firmware/SLC_UG.pdf`: ch.5 "Logging Out" (p.46–47) and paging paragraph + `set cli terminallines` (p.49–50); ch.6 services `Timeout` for Telnet/SSH; Quick Setup hostname note: "The hostname will be shown in the CLI prompt."
- `vendor_firmware/ext/slcupdate-5.2a/www/help/Services.htm`: "If you enable Telnet logins, you can cause an idle connection to disconnect after a specified number of minutes."
- `transcript.txt` (repo root) — the login-failure artifact explained in §1.
