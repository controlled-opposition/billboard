# Session

`bb` reuses one Blackboard session that a human creates with `bb auth login`. `bb auth status` reports whether that session works now. Every other command renews an expired session in headless Chrome when it can, and exits 4 when a human has to log in again.

## Sub-features

- `auth-status-ok` prints one status row and exits 0 when the session works.
- `auth-status-fields` exposes `loggedIn`, `user`, `name`, `expiresAt`, and `canRenew` through `--json`.
- `auth-no-renew` exits 4 instead of opening Chrome when `BB_NO_RENEW=1` is set and the session has expired.
- `auth-renew` renews an expired session headlessly and then completes the command.

## How to get to it (user POV)

- Run `bb auth status`.
- Run any other command. It checks and renews the session first.
- Set `BB_NO_RENEW=1` to forbid renewal.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.

- **Status row.** Check the session. Run `bbv run auth-status auth status --json loggedIn,expiresAt,canRenew`. Exit 0, and stdout is a one-element array with `loggedIn: true` and an `expiresAt` in the future.
- **Text row.** Check the default output. Run `bbv run auth-status-tsv auth status`. Exit 0, and stdout is one tab-separated row that starts with `true`.
- **No renewal, live session.** Run `BB_NO_RENEW=1 bbv run auth-norenew auth status --json loggedIn`. Exit 0 while the session is still valid. This proves only that the variable does not break a live session.
- **Renewal.** Only possible after the session expires on its own. When `bbv doctor` shows an `expiresAt` in the past and `canRenew: true`, run `bbv run auth-renew auth status --json loggedIn,expiresAt` without `BB_NO_RENEW`. Exit 0 and a new `expiresAt` in the future prove renewal. Exit 4 means renewal failed: report it, and ask the human to log in.

## Gotchas

- Never run `bb auth login` or `bb auth logout`. Login needs the human's NetID and MFA, and logout deletes their session.
- There is no safe way to force an expiry. `auth-no-renew` with an expired session and `auth-renew` can only be proven when the session has run out by itself. Report them as skipped otherwise.
- `bbv doctor` always sets `BB_NO_RENEW=1`. Run the renewal journey through `bbv run`, which does not set it.
- As of 2026-10-14, headless renewal has never been observed on the live site. A pass is new evidence. Record the run id.
- `cas.wm.edu` returns HTTP 403 to a user agent containing `HeadlessChrome`. If renewal fails with a 403, check the user-agent override first.
- `user` and `name` are personal data. Leave them out of reports.
