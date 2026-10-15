# Raw API

`bb api <path>` sends a GET to a path on blackboard.wm.edu with the stored session and prints the raw response body. It is the escape hatch for anything the typed commands do not cover. Renewal, pacing, the request cap, and exit codes work as in every other command.

## Sub-features

- `api-get` prints the response body of a REST path.
- `api-jq` filters the response with `--jq`.
- `api-relative` treats a path without a leading slash as relative to the host root.
- `api-notfound` exits 5 for HTTP 404.

## How to get to it (user POV)

- Run `bb api /learn/api/public/v1/...`.
- Add `--jq EXPR`.

## Driving it with bbv

Preconditions:

- `bbv doctor` prints `session ok`.

- **Current user.** Run `bbv run api-me api /learn/api/public/v1/users/me --jq '.id | type'`. Exit 0 and stdout `string`. `--jq` prints strings raw, without quotes. The check avoids printing personal data.
- **Filter.** Run `bbv run api-terms api /learn/api/public/v1/terms --jq '.results | length'`. Exit 0 and a number.
- **No leading slash.** Run `bbv run api-noslash api learn/api/public/v1/terms --jq '.results | length'`. Exit 0 and the same number as `api-terms`.
- **Not found.** Run `bbv run api-404 api /learn/api/public/v1/courses/_999999999_1`. Exit 5, empty stdout, and stderr `bb: HTTP 404: Cannot find the resource requested`.

## Gotchas

- A full URL is not refused. `bb api https://example.com/x` becomes the path `/https://example.com/x` on Blackboard, which answers with the Ultra HTML page and exit 0. The cookie does not leave the host, but the mistake is silent. Observed on 2026-10-14.
- Unknown paths outside `/learn/api` can also return the Ultra HTML page with HTTP 200, so exit 0 alone does not prove the path exists. Check that stdout is JSON.
- Small ids such as `_1_1` exist on Blackboard (`_1_1` is the SYSTEM course). Use `_999999999_1` for a missing id.
- `bb api` sends only GET. There is no flag for other methods.
- Responses contain personal data. Filter with `--jq` to the shape you need before printing.
