---
name: verify
description: Drive the real bb CLI against the live blackboard.wm.edu with the stored session, and keep a transcript of each command as proof. Use after changing a bb command, its output, or its exit codes, to show the change works on the real site and not only in go test.
---

# Verify bb against the live site

`bb` is a short-lived CLI. Nothing stays running. A verification run builds `bb` from the working tree, checks the stored session, runs `bb` commands the way an agent or a person would, and records each command's stdout, stderr, and exit code.

The helper is `.claude/skills/verify/scripts/bbv`. Run it from anywhere in the repository. Each call is independent, so it works from a shell that does not keep state between commands.

```sh
.claude/skills/verify/scripts/bbv start                      # build into a new run
.claude/skills/verify/scripts/bbv doctor                     # build current? session valid?
.claude/skills/verify/scripts/bbv run LABEL course list      # run `bb course list`, keep the proof
.claude/skills/verify/scripts/bbv scratch                    # directory for --dir
.claude/skills/verify/scripts/bbv cleanup                    # drop binary and scratch, keep proof
.claude/skills/verify/scripts/bbv where                      # print the run directory
```

## Rules for the live site

These come from `AGENTS.md`. They hold for every step below.

- Use read commands only. `bb` sends nothing but GET and HEAD.
- Keep each check under about 80 requests. `bbv run` adds `--max-requests 80` unless you pass your own value.
- Never run `bb auth login` or `bb auth logout`. Login needs the human's NetID and MFA. Logout deletes their session.
- If a command exits 4, stop and ask the human to run `bb auth login`. Do not retry.
- The evidence holds real course codes, names, and ids. It lives outside the repository, in `~/.cache/bb-verify/`. Never copy it into the repository, a commit, a PR, or an issue. Describe results without the data, for example "exit 0, 7 rows".

## Launch

```sh
.claude/skills/verify/scripts/bbv start
```

This runs `nix develop -c go build -o <run>/bin/bb ./cmd/bb` on the working tree, uncommitted changes included. It creates `~/.cache/bb-verify/<run id>/` with `bin/`, `scratch/`, and `evidence/`, and makes that run the current one. Ready means the output ends with `dir <path>`. If the build fails, the error is printed and nothing is marked current.

Use `go build` and not `nix build` here. `nix build` sees only files that git tracks, so it silently drops new untracked files.

Two agents can verify at the same time. Each one sets `BBV_RUN=<run id>` on its `bbv` calls after `start`, so neither uses the other's run. Concurrent `bb` processes are safe, because `auth.Manager` locks the session for every write.

## Doctor

```sh
.claude/skills/verify/scripts/bbv doctor
```

Run it before driving anything, and again whenever a result looks wrong. It is read-only and sends one request. It sets `BB_NO_RENEW=1`, so it never opens Chrome.

| Line | Meaning | Action |
| --- | --- | --- |
| `build ok` | The binary matches the working tree. | Continue. |
| `build STALE` | The code changed after `bbv start`. | Run `bbv start` again. |
| `session ok` | The stored session works now. | Continue. |
| `session EXPIRED` | `bb auth status` exited 4. | Stop. Ask the human to run `bb auth login`. |
| `session ERROR` | Any other exit code. | Read the code in the EXIT CODES table of `bb --help`. Code 7 means Blackboard is down; retry later. |

The `session` row also shows `expiresAt`. If that time is less than 10 minutes away, a later command may need a renewal. See the `auth` feature.

## Drive

```sh
.claude/skills/verify/scripts/bbv run LABEL <bb arguments>
```

`bbv run` runs `bb <bb arguments>` with the scratch directory as its working directory. It prints the command, the exit code, the stdout line count, stdout, and stderr. Exit 4 adds a reminder to stop. LABEL is a short name for the evidence files, such as `due-default`.

Find real arguments at run time from `bb` itself. `--jq` and `--template` need `--json` with the fields they read. Do not write course codes or ids into a feature file or a commit. To get a current course code:

```sh
.claude/skills/verify/scripts/bbv run pick-course course list --json code --jq '.[0].code'
```

stdout is not a terminal under `bbv`, so `bb` prints tab-separated rows without a header. To check the table that a person sees, run the binary under a pseudo-terminal: `script -q /dev/null "$(.claude/skills/verify/scripts/bbv where)/bin/bb" course list`. That command records no evidence, so describe what you saw.

Read [`features/README.md`](features/README.md) first, then follow the file for the feature you changed. Drive every entry point that file lists, not only the most convenient one.

## Evidence

Each `bbv run` writes four files to `~/.cache/bb-verify/<run id>/evidence/`. NNN is a sequence number:

- `NNN-LABEL.cmd` holds the command line.
- `NNN-LABEL.out` holds stdout.
- `NNN-LABEL.err` holds stderr.
- `NNN-LABEL.exit` holds the exit code.

A proof meets these standards:

- It runs the real binary against the real site. `go test` runs against a fake server, so it is not a substitute.
- It records the action and the resulting state. For downloads, also list the files on disk with `find "$(.claude/skills/verify/scripts/bbv scratch)" -type f` and say what it showed.
- It checks the exit code, and stdout or stderr. A run with the right output and the wrong exit code fails.
- For `--dry-run`, it confirms that nothing was written: the scratch directory has no new files. A dry run still walks the course over the network, so it is not free.

Report each result with its feature ID, its evidence file, and its exit code. Report a step you could not run as skipped, with the reason. Do not count another path as proof for it.

## Cleanup

```sh
.claude/skills/verify/scripts/bbv cleanup
```

This deletes `bin/` and `scratch/` of the current run, including every downloaded file. It keeps `evidence/`. There is no process to stop. Cleanup never touches the Keychain item or the Chrome profile.

Run cleanup after every run, including failed ones. Downloaded course files must not outlive the run. Old evidence can be deleted with `rm -rf ~/.cache/bb-verify/<run id>` once nobody needs it.

## Offline checks

`nix develop -c go test ./...` runs every command against a synthetic Blackboard. Run it before a live run. It needs no session, and it covers the error paths that the live site cannot produce on demand, such as a 5xx response or a failed renewal.
