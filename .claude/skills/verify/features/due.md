# Due dates

`bb due list` shows graded items with a due date in a time window, from the Blackboard calendar, sorted by due time. Items from dropped courses are left out. Each item carries a submission `status` from the user's gradebook, as the fifth default column.

## Sub-features

- `due-default` lists items due from one day ago to 14 days ahead.
- `due-status` gives every item a `status` from the gradebook, at one request per course with an item in the window.
- `due-pending` keeps only `unsubmitted` and `unknown` items with `--pending`.
- `due-window` accepts `--since` and `--until` as RFC 3339, a date, or a signed offset.
- `due-long-window` accepts a window longer than the calendar's 16-week limit.
- `due-course` filters to one or more courses with `--course`.
- `due-bad-time` exits 2 for a time it cannot parse, before any request.

## How to get to it (user POV)

- Run `bb due list`.
- Run `bb due list --since <time> --until <time>`.
- Run `bb due list --course COURSE`, repeated for several courses.
- Run `bb due list --pending`.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.
- The account has at least one current course. Some items in the next few weeks make the proofs stronger, but an empty result is still exit 0.

- **Default window.** Run `bbv run due-default due list --json due,title,kind,course`. Exit 0, a JSON array sorted by `due`, and every `due` between one day ago and 14 days ahead.
- **Status.** Run `bbv run due-status due list --since -14d --until 14d --json title,kind,attemptable,status`. Exit 0, and every element has a `status` that `bb due list --help` lists. A `submitted` or `graded` item should match what Ultra shows for it. Run `bbv run due-status-tsv due list --since -14d --until 14d` and check that each line has five tab-separated fields.
- **Pending.** Run `bbv run due-pending due list --since -14d --until 14d --pending --json title,status`. Exit 0, and every `status` is `unsubmitted` or `unknown`. Each one also appears in `due-status` with the same status.
- **Explicit window.** Run `bbv run due-window due list --since -3d --until 7d --json due`. Exit 0 and every `due` inside that window.
- **Long window.** Run `bbv run due-long due list --since -1d --until 200d --json due,title`. Exit 0, no error on stderr, and at least as many items as `due-default`.
- **One course.** Take a `course.code` from `due-long`. Run `bbv run due-course due list --since -1d --until 200d --course <code> --json course`. Exit 0 and every element's `course.code` equals that code.
- **Bad time.** Run `bbv run due-bad due list --since yesterday`. Exit 2, empty stdout, and stderr `bb: invalid time "yesterday": use RFC 3339, YYYY-MM-DD, or an offset like -1d, 14d, 36h`.

## Gotchas

- The calendar API rejects windows longer than 16 weeks. `bb` splits them. A long window that fails points at the split, not at Blackboard.
- `due` is UTC. Compare it with UTC bounds, not local time.
- `kind: manual` columns are often in-class exams. They have no online attempt, so their status is `none` until they are graded.
- Exit 6 with rows on stdout means a course's gradebook could not be read. Its items show `unknown`, and stderr names the course. That is a real result, not a failed run.
- Each course with an item adds one request. A wide window over many courses still stays well under 80 requests.
- An empty array with exit 0 is a valid result for a quiet window. Widen the window before deciding a filter is broken.
