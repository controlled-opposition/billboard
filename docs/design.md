# Rationale: candidate 1, a deep `bb` package behind a thin CLI with schema-driven output

## Problem

`bb` is a gh-style CLI for W&M Blackboard Ultra. Its main user is an AI agent working from a shell. The agent never sees a prompt. It reads `--help` as documentation, branches on exit codes, and parses TSV or `--json`. Four facts from grounding make the shape non-obvious:

- Login requires a human with MFA. The agent can only reuse that session, or renew it headlessly from the Chrome profile, and renewal is unproven. "Needs a human" must therefore be a first-class outcome.
- Several `bb` processes can run at once, and they all share the keychain item and the Chrome profile.
- Ultra stores data in ways the API does not describe. Documents are folders that contain a hidden `ultraDocumentBody` child. Files are `<a data-bbfile>` tags inside HTML bodies, and their bbcswebdav URLs are signed and expire. Course names are HTML-escaped.
- Due dates come from only one source that worked for every course (the public `calendars/items`). Gradebook columns return 403 in 3 of 8 courses. No probed endpoint reports whether an item was submitted.

There is also a shared budget of 10k requests per day, and a full walk of one course takes about 60 requests.

## Usage (caller's view)

```
$ bb auth status; echo $?
true	et	2026-10-15T03:59:00Z
0
$ bb course list
MATH101-01-F26	Calculus & Analysis	Fall 2026
...
$ bb due list --since -1d --until 10d
2026-10-15T03:59:00Z	MATH101-01-F26	Problem Set 3	test
$ bb due list --course math101 --json title,due,kind --jq '.[] | select(.kind=="test") | .title'
Problem Set 3
$ bb content list math101 --files-only --json id,path,files
[{"id":"_20001_1","path":["Week 3","LLM use"],"files":[{"name":"Tips ... .pdf","size":250000,"mimeType":"application/pdf"}]}]
$ bb content download math101 _20001_1 --dir ~/bb
downloaded	~/bb/MATH101-01-F26/Week 3/LLM use/Tips ... .pdf
$ bb content download math101 _20001_1 --dir ~/bb      # rerun: no download request
skipped	~/bb/MATH101-01-F26/Week 3/LLM use/Tips ... .pdf
$ bb due list; echo $?                                    # session dead, renewal failed
bb: blackboard login required: a human must run `bb auth login`
4
```

The agent's loop has three steps. First, it runs `bb due list` or `bb content list`. Second, it picks ids from the output. Third, it runs `bb content download COURSE ID...`. The download step re-walks only the subtrees of the chosen ids, so it costs a few requests, not 60. Exit 4 means "ask the human". Exit 5 means the course argument was wrong, and the candidates are printed on stderr. Exit 6 means some files failed. Exit 7 means "retry later".

Go call sites inside `internal/cli` show the whole public surface a command uses:

```go
all, picked, err := coursesAndResolve(ctx, c, f.courses)          // 1 paged call, pure resolution
items, err := c.Due(ctx, all, bb.DueFilter{Window: w, Courses: ids(picked)})
return output.Print(e.streams, dueSchema, opts, items)

nodes, err := c.Walk(ctx, course.ID, bb.WalkOptions{Under: id})
for _, p := range bb.Plan(course, nodes) { results = append(results, c.Fetch(ctx, p, dir, force)) }
```

## Shape

```
cmd/bb/main.go            os.Exit(cli.Run(args))
internal/cli/             thin shell: flags -> bb -> output.Print; exit.go is the only error->code map
internal/output/          Schema[T]/Field[T]: one list derives --json names, validation, TSV/table columns, --help JSON FIELDS
internal/bb/              deep module: Client + domain types; one file per noun (course, due, content, download)
internal/auth/            Session (unforgeable), Manager (keychain + Chrome profile + flock), chromedp capture
```

Any command can be traced through three files: `cli/<cmd>.go`, then `bb/<noun>.go`, then `bb/client.go`.

**Data first.** The domain types are `Course`, `DueItem`, `ContentNode`, `Attachment`, `Planned`, and `FetchResult`. `ContentNode` is a flat row in depth-first order that carries `Path` and `Depth`. It is not a pointer tree. Every access pattern is a flat scan: list the rows, filter to nodes with files, plan paths, download. A flat slice serves all of these with no index. Named id types (`CourseID`, `ContentID`, `ColumnID`) stop an id of one kind from being passed where another is expected (per type-system-discipline).

**Boundary.** Each `bb/<noun>.go` file holds that noun's unexported wire struct, a pure `parse*`, `fold`, or `extractAttachments` function, and the public method. HTML unescaping, time parsing, the `ultraDocumentBody` fold, and `data-bbfile` decoding all happen there, once (per boundary-discipline). The only wire access left is `Client.Raw`, which returns bytes for `bb api`.

**One HTTP path.** `Client.do` owns this policy, in order:
1. Refuse writes unless the client was built with `AllowWrites`.
2. Pace requests through the budget.
3. Attach the session.
4. On a 401, renew once and retry once. A second failure returns an error that wraps `auth.ErrNeedsHuman`.
5. Map statuses to `APIError.Kind`.

No new method can skip this policy, because no other path to the network exists (per encode-lessons-in-structure).

**Shared state across processes.** The keychain item and the Chrome profile are shared by concurrent `bb` processes. Separating them per process is impossible, because there is one Blackboard login. That makes them a real one-writer invariant. `auth.Manager` is the only writer, and every write runs under a flock. `Renew(stale)` re-reads the keychain after taking the lock and returns the newer session if another process already renewed it. Two agents that hit a 401 together therefore launch Chrome once (per separate-before-serializing-shared-state). Reads take no lock.

**Invariants in types:**
- `auth.Session` has only unexported fields and one parsing constructor, so a session without XSRF cannot exist.
- `bb.Window` can only be built by `NewWindow`, so `until > since` always holds.
- `Attachment.href` is unexported, so the signed, expiring URL is never printed or stored as an identifier. `RID` is the stable identifier.
- `ResolveCourse` is pure, works on a list that has already been fetched, and never guesses. An ambiguous argument becomes `KindAmbiguous` with candidates.

**Idempotency.**
- Download paths are deterministic: course code, then the outline path, then the file name. Segments are sanitized, and collisions are broken by RID, never by walk order.
- `Fetch` skips a file that already exists with the expected size, and writes through a temp file plus rename. Reruns and crashes therefore converge (per make-operations-idempotent).
- `auth login` and `auth logout` are both idempotent.

**Interface depth.** A command sees about seven calls: `Courses`, `ResolveCourse`, `Due`, `Walk`, `Plan`, `Fetch`, and `Raw`. Behind them sit paging, chunking of the calendar window, renewal with cross-process deduplication, the Ultra document fold, HTML attachment extraction, status mapping, and pacing. Output is one generic `Print` driven by a declarative schema. Adding a field means adding one line, and that line updates the help text too.

**Deliberately not done.**
- No disk cache.
- No concurrency inside a process, so pacing stays trivial and the budget stays safe.
- No submission status.
- No private `/learn/api/v1` endpoints except through `bb api`.

## Synthesis decision

Arena on 2026-10-14 with three runners (Opus, Fable, Sonnet) and a Fable cross-judge. All three converged on the same overall shape: one deep `bb` client, schema-driven output, one error-to-exit-code map, and a single-writer `auth.Manager` with flock-guarded renewal. That convergence is the main evidence for the shape.

**Base: candidate 1 (Opus).** Judge and orchestrator agreed. It has the only single HTTP path (`Client.do`) carrying the write gate, pacing, renewal and status mapping, an unforgeable `auth.Session`, and exit-code help derived from the same table as the switch.

**Fixes to the base before grafting.**
- `auth.Manager.Current` never renews or launches Chrome. Only `Renew` does.
- Every enum shown in help text (fetch status, due kind, exit codes) is derived from its constants, never retyped. This also removes the drifted "planned" status.

**Grafts.**
- From candidate 2: an `Execute(ctx, env, args) int` entry point with injectable stdout, stderr and `BaseURL`, so behavior tests drive the real CLI against an `httptest` server.
- From candidate 2: files are extracted from the `ultraDocumentBody` child already present in the folder's children response, so finding files costs no extra request.
- From candidate 2: `Fetch` re-walks the owning document and retries once when the signed bbcswebdav URL returns 403 or redirects to login.
- From candidate 3: `Plan` stats existing files through a small filesystem seam, so `content download --dry-run` prints the real plan including skips.
- From candidate 3: a per-process hard request cap (`--max-requests`, default 300), enforced in `Client.do`.
- From candidate 3: a distinct exit code for forbidden (403) apart from not-found, so an agent can tell "no access" from "bad id".

**Fixes for defects all three shared.**
- Skip rule: a local file is current when its size equals the attachment's `fileSize` and its mtime equals the content item's `modified` time. `Fetch` sets the mtime after the rename. An unknown size falls back to the mtime check alone.
- Path convergence: a subtree download resolves ancestor titles by following `parentId` up to ROOT (one GET per ancestor), so `download COURSE ID` and a full download write the same file to the same path.
- Renewal fails fast. If headless renewal lands on an Entra page that asks for credentials or MFA, it stops at once and returns the needs-login error (exit 4). It does not wait out the timeout. Renewal has a 20 s ceiling, and `BB_NO_RENEW=1` disables it.
- Fixtures: tests use synthetic JSON shaped like the captured responses. No captured personal data enters the repo.

**Rejected.**
- Candidate 3's always-on gradebook enrichment. It costs one request per course and returns 403 for 3 of 8 W&M courses. It may come back as `bb grade list`.
- Candidate 3's `Backend` interface, a hand-synced copy of `Client`. Tests use `httptest` through `BaseURL`.
- Candidate 2's exported `bb.Session` cookie fields, which leak the wire format.
- Keychain fallbacks for other OSes. v1 is macOS-only, and go-keyring already covers Linux Secret Service if it is ever needed.

## Tradeoffs accepted

- We accept that every folder costs a children request in `Walk` in exchange for correct document folding. A folder can only be identified as an Ultra document from its children. The `Under` option plus the list-then-download-ids loop keeps the common case cheap.
- We accept a pure `ResolveCourse` that needs the course list passed in (one extra request per command) in exchange for zero hidden fetches. This also gives the `Course` join that due items need.
- We accept leaving submission status out of `DueItem`, which may look like an oversight. Every value would be "unknown" today. Adding the field later is non-breaking.
- We accept sequential, paced requests (a full course download is about 15s of walking) in exchange for no shared mutable state in the client and gentle use of the budget.
- We accept a generic `output.Schema[T]` with `func(T) any` getters (losing some type safety at render time) in exchange for one source of truth across `--json`, TSV, the table view, and `--help`.
- We accept that the default TSV view drops nested fields (`course` renders as its code, `files` as a count) in exchange for stable, awk-able lines. `--json` carries the full structure.

## Alternatives considered

- **Generic resource layer.** This would be an endpoint catalogue with templates (as in blackboard-mcp) plus raw JSON passed through jq. It is shallow: the agent would have to learn Ultra's folder/`ultraDocumentBody` trick, entity-escaped `data-bbfile`, and HTML-escaped names. That is exactly the knowledge `bb` exists to hide. It survives only as `bb api`.
- **Persistent local index.** A sync command would mirror courses, the content tree, and the calendar into SQLite, and every command would query that mirror. That makes list and download cheap and allows offline use. It adds a second source of truth with staleness rules. The signed file URLs expire, so downloads would need a re-walk anyway. It also gives the agent a "did you sync?" failure mode. It lost on the costs of keeping state in sync.
- **Interface-per-resource services** (`CourseService`, `ContentService`, and so on, as in go-github). This adds a layer that only passes calls through, with one implementation per interface. A single concrete `Client` with methods grouped by file gives the same discoverability with fewer hops.

## Implementation reconciliation

- Blackboard does not HTML-escape course names or titles; the `&amp;` seen during grounding was a display artifact. API strings pass through unchanged, and entity decoding happens only where HTML is parsed (the `data-bbfile` attribute in document bodies).
- A bbcswebdav file downloads with the cookie session alone at its path without the signed query string (verified live: the full PDF). `Attachment` stores that unsigned path, which does not expire, so the "re-walk and retry once on an expired signed URL" graft is dropped.
- `calendars/items` rejects ranges over 16 weeks with HTTP 400 ("since and until cannot span more than 16 weeks"); `Due` splits wider windows into 16-week requests and dedupes items by id. The default window is one day back through 14 days ahead.
- `due list` leaves out items from dropped courses (membership availability "Disabled"), the same courses whose gradebook columns return 403.
- Some `data-bbfile` links lack `data-bbtype="attachment"`, `fileName`, and `fileSize` (seen on a syllabus). Any `<a data-bbfile>` pointing at `/bbcswebdav/` counts; the name falls back to `displayName` then `linkName`, and with no size the skip rule compares mtime alone.
- The skip-rule mtime is the `modified` time of the body that links the file (the `ultraDocumentBody` child), not the folder's, which changes whenever the outline is reordered.
- `content download` with no ids requires `--all`. `--match` and `--force` and `WalkOptions.Depth` are not built.
- An HTML page served where a non-HTML file was expected is a failed file (exit 6), not exit 4: the walk that just succeeded proves the session works.
- The HTTP client bounds the wait for response headers (20 s) instead of the whole exchange, so large files are not cut off, and it drops `X-Blackboard-XSRF` on a redirect to another host.

## Open questions and risks

- **Riskiest: does a bbcswebdav URL download with the cookie session, and does it outlive the walk?** If the signature (`VxJw3wfC56` looks like an epoch expiry) is short-lived, the design survives, because each download re-walks just before fetching. If the cookie session gets a 302 to a login page, `open` reports that as exit 4. If a different auth path is needed, `download.go` alone changes. Can someone test one URL manually before `Fetch` is built?
- Headless renewal is unproven. If Entra asks for MFA again on each renewal, exit 4 becomes the common case. Is that acceptable for a v1?
- W&M terms have no dates, so `isCurrent` parses the season and year from the term name ("Fall 2026"). Is a hard-coded date table per season acceptable, or should "current" mean courses that have calendar items or content modified this term?
- Does `calendars/items` cap the window size? The design chunks the window internally, but the cap is unknown.
- The private `/learn/api/v1/courses/{id}/contents/ROOT/children` response was 3x larger than the public one. It may expose the document body inline and remove the per-folder requests. Worth probing before `Walk` is built.
- Should `bb content download` with no ids require `--all`? This would stop an agent from casually spending about 60 requests.

## Next implementation step

Build `bb/content.go`'s pure `fold` and `extractAttachments` against synthetic fixtures that match the shapes captured in the prototype. They are the least certain domain logic and need no network.
