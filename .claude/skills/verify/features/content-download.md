# Content download

`bb content download` saves the files linked from chosen content items, or from a whole course with `--all`, to `DIR/<course code>/<outline path>/<file name>`. A dry run prints the plan. A re-run skips files that are already complete on disk.

## Sub-features

- `download-dry-run` prints each file as `planned` and writes nothing.
- `download-items` downloads the files under the given ids and reports `downloaded`.
- `download-layout` places each file at `DIR/<course code>/<outline path>/<file name>`.
- `download-rerun` reports `skipped` for a file already on disk, without fetching it.
- `download-partial` exits 6 when some files fail and still attempts the rest.

## How to get to it (user POV)

- Run `bb content download COURSE ID... --dir DIR`.
- Run `bb content download COURSE --all --dir DIR`.
- Add `--dry-run` to either form.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.
- A `document` id with at least one file, found through [content-list](./content-list.md) with `--files-only`. Prefer the smallest one.

- **Dry run.** Run `bbv run dl-dry content download <code> <doc id> --dir dl --dry-run --json status,path,size,mimeType`. Exit 0 and every element has `status: planned`. Then run `find "$(.claude/skills/verify/scripts/bbv scratch)/dl" -type f`. It prints nothing.
- **Download.** Run `bbv run dl content download <code> <doc id> --dir dl --json status,path,mimeType`. Exit 0 and every element has `status: downloaded`. Then run `find "$(.claude/skills/verify/scripts/bbv scratch)/dl" -type f -exec stat -f '%z %N' {} +`. Each `path` from stdout exists, sits under `dl/<course code>/`, and is larger than 0 bytes. Run `file -b` on it; the type matches `mimeType`.
- **No leftovers.** Run `find "$(.claude/skills/verify/scripts/bbv scratch)/dl" -name '*.part'`. It prints nothing.
- **Re-run.** Run the download command again as `bbv run dl-again ...`. Exit 0 and every element has `status: skipped`.
- **All files.** Run `bbv run dl-all-dry content download <code> --all --dir dl --dry-run --json status`. Use a course whose walk fits in the budget. Exit 0 and every element is `planned` or `skipped`. Do not run `--all` without `--dry-run`: it downloads every file in the course.

## Gotchas

- `--dry-run` writes nothing but still walks the outline over the network. It is not free.
- The `size` field shows the size from the outline, which is 0 when the link has none. It stays 0 even after a download, so check the size on disk with `stat`, not with `size`. Observed on 2026-10-14.
- `download-partial` needs a file that fails. The live site rarely provides one. `go test` covers it, so report the live journey as skipped unless a real failure occurs.
- `bbv cleanup` deletes everything under `dl`. Record the file listing in the report before cleanup.
- Downloaded files are course material. They must never enter the repository.
