# bb verification map

This directory is the maintained list of what `bb` does for its user, and how to prove each behavior on the live site. Read this index, then follow the file for the feature you changed. The helper and the evidence layout are in [`../SKILL.md`](../SKILL.md).

## Baseline preconditions

- `bbv start` built the binary from the current working tree.
- `bbv doctor` prints `build ok` and `session ok`.
- The account has at least one current course. Some journeys also need a course with an Ultra document that links a file.
- No step runs `bb auth login` or `bb auth logout`.

## Driving conventions

- Run every `bb` command through `.claude/skills/verify/scripts/bbv run LABEL ...`. This document writes it as `bbv run`.
- Find course codes and content ids at run time with `bb`. Never write a real code, id, title, or name into this map.
- Assert on `--json` output. TSV and table output are for checking layout only.
- Downloads go under the run's scratch directory. `bbv run` already uses it as the working directory, so `--dir dl` writes to `<scratch>/dl`.
- Keep each check under about 80 requests. A whole-course walk can cost 60 or more requests.

## Proof and skip reporting

- A proof names the feature ID, the evidence file (`NNN-LABEL`), and the exit code.
- A command proof checks stdout or stderr and the exit code together.
- A download proof also lists the files on disk.
- If the account lacks a precondition, report the journey as skipped and give the reason. Do not drive a different path and call it the same proof.
- Exit 4 ends the run. Ask the human to run `bb auth login`.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph about the behavior the user sees. It then has exactly four H2 sections, in this order:

1. `Sub-features` lists short IDs, with one line for each behavior.
2. `How to get to it (user POV)` lists every command and flag that reaches the feature.
3. `Driving it with bbv` starts with `Preconditions:`, then pairs each user action with an exact command and the result to observe.
4. `Gotchas` lists traps that waste a run or make its proof wrong.

## Features

- [Session](./auth.md) covers `bb auth status`, the exit-4 contract, and headless renewal.
- [Courses](./courses.md) covers listing, `--all`, resolving a COURSE argument, and the output formats.
- [Due dates](./due.md) covers the default window, `--since` and `--until`, long windows, `--course`, submission status, and `--pending`.
- [Content outline](./content-list.md) covers whole-course and subtree listing and `--files-only`.
- [Content download](./content-download.md) covers dry runs, downloads, re-runs, and the file layout.
- [Raw API](./api.md) covers `bb api` and its path handling.
