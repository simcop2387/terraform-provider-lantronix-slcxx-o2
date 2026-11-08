# Issue tracker: GitHub (via MCP)

Issues and specs for this repo live as GitHub issues in `simcop2387/terraform-provider-lantronix-slcxx-o2`.

**Primary interface: the GitHub MCP server tools** (`github_issue_read`, `github_issue_write`, `github_sub_issue_write`, `github_list_issues`, ...). The `gh` CLI is NOT usable here: its stored token is invalid and the token in `secrets.txt` lacks API scopes (it works for `git push` over HTTPS only). Never print the token; extract it with `sed -n 's/^github token: //p' secrets.txt` if needed.

## Conventions

- **Create an issue**: `github_issue_write` with `method: "create"`, `owner: simcop2387`, `repo: terraform-provider-lantronix-slcxx-o2`.
- **Read an issue**: `github_issue_read` with `method: "get"` (comments via `method: "get_comments"`).
- **List issues**: `github_list_issues` (filter by `labels`, `state`).
- **Make an issue a sub-issue of a parent**: `github_sub_issue_write` with `method: "add"`, `issue_number` = parent number, `sub_issue_id` = the child's **database id** (the `id` field from the create response or `github_list_issues`, NOT the issue `number`).
- **Blocking**: GitHub native issue dependencies are not exposed by the MCP tools available here, so use the body convention: a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed. (If `gh` auth is ever fixed, prefer native dependencies instead and update this file.)
- **Comment on an issue**: `github_issue_write` has no comment method; use `github_add_issue_comment`.
- **Apply / remove labels**: `github_issue_write` with `method: "update"` and `labels` (full replacement list). Labels must exist first; create them in the repo UI or via `curl -H "Authorization: Bearer $(sed -n 's/^github token: //p' secrets.txt)" -X POST https://api.github.com/repos/simcop2387/terraform-provider-lantronix-slcxx-o2/labels -d '{"name":"...","color":"ededed"}'` if missing.
- **Close**: post the resolution with `github_add_issue_comment`, then `github_issue_write` with `method: "update"`, `state: "closed"`.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Read it with `github_issue_read`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Destination / Notes / Decisions-so-far / Not-yet-specified / Out-of-scope body.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue via `github_sub_issue_write` (needs the child's database id). Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev (`simcop2387`).
- **Blocking**: `Blocked by: #<n>, #<n>` line at the top of the child body (see **Blocking** above).
- **Frontier query**: `github_list_issues` with `state: "open"`, scoped to the map's sub-issues (`github_issue_read` `method: "get_sub_issues"` on the map); drop any with an open blocker or an assignee; first in map order wins.
- **Claim**: `github_issue_write` `method: "update"` with `assignees: ["simcop2387"]`, the session's first write.
- **Resolve**: resolution comment (`github_add_issue_comment`), close (`github_issue_write` `state: "closed"`), then append a context pointer (gist + link) to the map's Decisions-so-far.
