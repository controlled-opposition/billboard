# Content outline

`bb content list` prints a course outline depth-first, one row per item, as Ultra shows it. An Ultra document is one row that carries the files linked from its body. With an ID, it lists only that item and everything under it.

## Sub-features

- `content-course` lists the whole outline of a course.
- `content-subtree` lists one item and everything under it.
- `content-files-only` keeps only items with at least one file.
- `content-foreign-id` exits 3 for an id that does not belong to the course.
- `content-document-fold` shows an Ultra document as one `document` row with its files.

## How to get to it (user POV)

- Run `bb content list COURSE`.
- Run `bb content list COURSE ID`.
- Add `--files-only` to either form.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.
- A current course whose outline fits in the request budget, or a top-level folder id in a larger course.

- **Pick a course.** Run `bbv run pick-course course list --json code,id`. Choose a course.
- **Top-level folders without a walk.** Run `bbv run top api '/learn/api/public/v1/courses/<id>/contents?fields=id,title,contentHandler.id'`. Exit 0 and a `results` array. This costs one request, so use it to choose a folder in a large course.
- **Whole course.** Run `bbv run content-course content list <code> --json id,kind,path,depth,files`. Exit 0, a depth-first array with `depth` 0 for top-level rows, and no row titled `ultraDocumentBody`. If it exits 1 with `request cap reached`, the course is too large; use the subtree journey instead.
- **Subtree.** Take a folder `id` from `top`. Run `bbv run content-subtree content list <code> <folder id> --json id,kind,depth,parentId`. Exit 0, the first row is the folder, and every other row descends from it.
- **Files only.** Run `bbv run content-files content list <code> <folder id> --files-only --json id,kind,files`. Exit 0, and every row has a non-empty `files` array. An empty array means this folder has no files; try another folder before calling it a failure.
- **Foreign id.** Run `bbv run content-foreign content list <code> _1_1`. Exit 3, empty stdout, and stderr `bb: HTTP 403: The selected content does not belong to the given course`.

## Gotchas

- A whole-course walk costs one request per folder, about 60 for a large course. `bbv run` caps at 80. Hitting the cap exits 1 with `bb: request cap reached; raise it with --max-requests if this walk really needs more`.
- Some current courses have only a test or two and no files. Check `kind` counts before deciding `--files-only` is broken.
- `files[].size` is 0 when Blackboard's link does not carry a size. That is expected for older links.
- Ultra stores a document as a folder whose only child is `ultraDocumentBody`. A row with that title means the fold broke.
