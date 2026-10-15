# Grades

`bb grade list` shows every gradebook column the user can see, with a status, score, points possible, and display grade, sorted by course, then due time, then title. With no arguments it covers the current courses where the user is a student. Columns hidden from students are left out.

## Sub-features

- `grade-current` lists the gradebook of every current student course, at 2 requests per course.
- `grade-course` lists only the named courses, past ones included.
- `grade-status` gives each column a status that `bb grade list --help` lists, and never `unknown`.
- `grade-json` carries `score` and `due` as `null` when absent, and `feedback` as plain text.
- `grade-partial` prints the readable courses and exits 6 when some gradebooks answer 403 or 404.

## How to get to it (user POV)

- Run `bb grade list`.
- Run `bb grade list COURSE...`.
- Run `bb grade list --json FIELDS`, with `--jq` or `--template` if needed.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.
- The account has at least one current course where it is a student. A course with at least one graded item makes the score proofs stronger.

- **Current courses.** Run `bbv run grade-current grade list --json course,title,status,score,possible,kind`. Exit 0, a JSON array grouped by `course.code`, and every `status` is one of the statuses in the help. Each course from `bb course list` appears at least once unless its gradebook is empty.
- **TSV layout.** Run `bbv run grade-tsv grade list`. Exit 0 and every line has six tab-separated fields.
- **One course.** Take a `course.code` from `grade-current`. Run `bbv run grade-course grade list <code> --json course,title,status,score,possible,display,due,feedback`. Exit 0 and every element's `course.code` equals that code. A `graded` row with a score should match Ultra's grades page.
- **Totals.** Run `bbv run grade-totals grade list --json course,title,display,score,possible --jq '.[] | select(.title | test("Overall|Total"))'`. Exit 0. Rows that print are course totals; compare one with Ultra.
- **Status agrees with due list.** Run `bbv run grade-ids grade list --json columnId,status` and `bbv run grade-due due list --since -14d --until 14d --json columnId,status`. Exit 0 for both. For each `columnId` in both outputs, the statuses are equal. The one allowed difference is `unsubmitted` against `none`: `grade list` decides it from the column's grading type and `due list` from the calendar's `attemptable` flag. Report any such pair.

## Gotchas

- `bb grade list` with no arguments costs 3 requests plus 2 per current course, plus one per extra page of columns. Seven courses cost about 17.
- `score` is `null` for unsubmitted, waiting, exempt, and some graded items. A `graded` row with no score is a real Blackboard state.
- `kind: calculated` rows such as "Overall Grade" are totals. They often have a `display` letter and `counted: false`.
- Exit 6 lists the unreadable courses on stderr. Exit 3 or 5 with no stdout means no course could be read.
- `--jq` output escapes `&` in strings as a unicode escape. Compare decoded JSON, not raw text.
