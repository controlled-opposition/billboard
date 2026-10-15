# Courses

`bb course list` shows the courses the user is enrolled in for the current term, or every enrollment with `--all`. A COURSE argument is an internal id, a full code, or a code prefix, and the same resolution is used by `due`, `content list`, and `content download`.

## Sub-features

- `course-current` lists current-term courses.
- `course-all` lists every enrollment, including past, future, and closed ones.
- `course-resolve` prints only the course that each COURSE argument resolves to.
- `course-nomatch` exits 5 for a COURSE that matches nothing.
- `course-ambiguous` exits 5 and lists candidates on stderr for a prefix that matches several courses.
- `course-formats` prints TSV when piped, a table on a terminal, and JSON with `--json`, `--jq`, and `--template`.

## How to get to it (user POV)

- Run `bb course list`.
- Run `bb course list --all`.
- Run `bb course list COURSE...`.
- Pass a COURSE to `bb due list --course`, `bb content list`, or `bb content download`.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.
- The account has at least one current course.

- **Current courses.** Run `bbv run course-current course list --json code,current`. Exit 0, and every element has `current: true`.
- **All enrollments.** Run `bbv run course-all course list --all --json code,current`. Exit 0, at least as many elements as `course-current`, and some with `current: false` if the account has past terms.
- **Resolve by code.** Take a `code` from `course-current`. Run `bbv run course-resolve course list <code> --json code`. Exit 0 and exactly one element with that code.
- **Resolve by prefix.** Take the first 7 characters of that code, lowercased. Run `bbv run course-prefix course list <prefix> --json code`. Exit 0 and the same single course, unless another current course shares the prefix.
- **No match.** Run `bbv run course-nomatch course list zzzz999`. Exit 5, empty stdout, and stderr `bb: no course matches "zzzz999"; see `bb course list --all``.
- **Ambiguous prefix.** Find two codes in `course-all` that share a leading prefix and are both current, or both not current. Run `bbv run course-ambiguous course list <prefix>`. Exit 5 and the candidates on stderr. If no two codes qualify, report it as skipped.
- **TSV.** Run `bbv run course-tsv course list`. Exit 0 and tab-separated rows with no header.
- **Template.** Run `bbv run course-template course list --json code,name --template '{{range .}}{{.code}}{{"\n"}}{{end}}'`. Exit 0 and one code per line.

## Gotchas

- Prefix matching looks at current courses first and only falls back to all enrollments when no current course matches. A prefix shared by a current and a past course resolves to the current one, so it is not ambiguous.
- `--jq` and `--template` exit 2 without `--json`.
- Course names are not HTML-escaped. A display that shows `&amp;` escaped the text itself.
- `--jq` prints `&` as the escape `\u0026` in JSON strings. This is a known open bug, not a regression.
- Enrollments with availability `Disabled` are usually dropped courses. Blackboard returns HTTP 403 (exit 3) for their gradebook.
