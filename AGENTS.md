# AGENTS.md

`bb` is a command-line client for William & Mary's Blackboard Learn Ultra (https://blackboard.wm.edu), modeled on `gh`. Its main user is an AI agent that runs it from a shell. People use it too. This file is for agents that change the code. To use `bb`, read `bb --help` and `bb <command> --help`. The help text is the usage documentation, and the code generates it.

## Build and test

Go is not on `PATH` on the main development machine. Run Go through the flake's dev shell, which also enables the repository hooks:

```sh
nix develop -c go vet ./...
nix develop -c go test ./...
nix develop -c go build -o bb ./cmd/bb
```

With direnv, run `direnv allow` once and the shell loads on `cd`, so the `nix develop -c` prefix is not needed. `nix build` builds `bb` into `./result/bin/bb` and runs the tests. When `go.mod` or `go.sum` changes, `vendorHash` in `flake.nix` goes stale. Set it to `pkgs.lib.fakeHash`, run `nix build`, and paste the hash from the error.

The tests run the real commands through `cli.Execute` against an `httptest` server. They never touch the network.

## Layout

A command reaches the network through three files: `internal/cli/<command>.go`, then `internal/bb/<noun>.go`, then `internal/bb/client.go`.

| Path | Owns |
| --- | --- |
| `cmd/bb/main.go` | Calls `cli.Execute` and exits with its code. |
| `internal/cli/` | Cobra commands, flag parsing, output schemas, and the exit-code table in `exit.go`. |
| `internal/bb/` | The Blackboard client and the domain types. Wire structs and parsing stay unexported here. |
| `internal/auth/` | The session, the macOS Keychain item, the Chrome profile, and login and renewal through `chromedp`. |
| `internal/output/` | Table, TSV, `--json`, `--jq`, and `--template` output, driven by one `Schema` per record type. |
| `docs/design.md` | The design, the alternatives it rejected, and every accepted deviation. |

## Rules the code depends on

- `Client.do` in `internal/bb/client.go` is the only path to the network. It applies the write gate, request pacing, the `--max-requests` cap, the session cookie, one renewal on HTTP 401, and the mapping from HTTP status to error kind. Do not add a second path.
- `auth.Manager` is the only writer of the Keychain item and the Chrome profile. It takes a file lock for every write, because several `bb` processes can run at once.
- `bb` is read-only. Only a typed method that a command enables with an explicit flag may send a request other than GET or HEAD.
- `bb` never prompts and never reads stdin. Commands take input from arguments and flags only.
- Each record type has one `output.Schema`. The schema produces the `--json` field names, the TSV and table columns, and the JSON FIELDS section of the help. To add a field, add it to the schema.
- `internal/cli/exit.go` holds the exit-code table. The error mapping and the EXIT CODES help section both come from that table. Exit 4 means a human must run `bb auth login`.
- Domain types never carry wire structs. Parse Blackboard JSON in `internal/bb` and return domain types.

When a change has to break one of these rules, update `docs/design.md` in the same change and say why.

## Blackboard facts that are easy to get wrong

- Ultra stores a document as a folder whose only child is titled `ultraDocumentBody`. `bb` folds the pair into one `document` row.
- Files are `<a data-bbfile>` links inside the document body HTML, not content attachments. `bb` fetches each file at its `/bbcswebdav/...` path without the signed query string. The session cookie is enough.
- Course names and titles are not HTML-escaped. If a terminal or chat shows `&amp;`, the display escaped it, not Blackboard.
- `calendars/items` rejects a window longer than 16 weeks.
- `cas.wm.edu` returns HTTP 403 to a user agent that contains `HeadlessChrome`. Headless Chrome must override its user agent.
- An enrollment with availability `Disabled` is usually a dropped course. Blackboard returns HTTP 403 for its gradebook.

## Keep personal data out of the repo

Tests use invented data that has the shape of real responses. Never commit a captured response, a real course code or id, a person's name, a cookie, or a downloaded file. Before you push, search the diff for course codes and ids from the account you tested with.

## Test against the live site

`bb` reuses the session stored in the macOS Keychain. To check a change against the real site:

- Use only read commands, and keep each check under about 80 requests.
- Never run `bb auth login`. It needs a human for the NetID and MFA steps.
- If a command exits 4, stop and ask the human to run `bb auth login`.
- Download files into a scratch directory and delete them when you finish.

## Commits and pull requests

Commit subjects follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/): `type(scope): summary`.

- The type is one of `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `build`, `ci`, `style`, or `revert`.
- The scope is optional. Write it in lowercase and name the area, such as `auth`, `due`, `content`, or `output`.
- Write the summary in the imperative mood, with no trailing period. The whole subject is at most 72 characters.
- Add `!` after the type or scope when a change breaks the CLI contract, such as a JSON field name or an exit code.

`scripts/check-commit-msg.sh` enforces these rules. To run it on every local commit, enable the repository hooks once per clone:

```sh
git config core.hooksPath .githooks
```

Squash `fixup!` commits before you merge. Pull request titles use the same format, because the title becomes the squash commit subject.
