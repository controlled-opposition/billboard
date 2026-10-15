#!/bin/sh
# Usage: check-commit-msg.sh FILE        check one message (commit-msg hook)
#        check-commit-msg.sh --range A..B check every commit subject in a range (CI)
set -eu

types='feat|fix|docs|refactor|test|chore|perf|build|ci|style|revert'
pattern="^($types)(\([a-z0-9][a-z0-9-]*\))?!?: [^ ].*[^. ]$"
max=72
range=no

check() {
	case $1 in
	'fixup! '* | 'squash! '* | 'amend! '*)
		[ "$range" = no ] && return 0
		echo "squash before merging: $1" >&2
		return 1
		;;
	esac
	if ! printf '%s\n' "$1" | grep -Eq "$pattern"; then
		echo "not a Conventional Commit subject: $1" >&2
		echo "want: type(scope): imperative summary, no trailing period; type is one of ${types}" >&2
		return 1
	fi
	if [ "${#1}" -gt "$max" ]; then
		echo "subject is ${#1} characters; keep it at most $max: $1" >&2
		return 1
	fi
}

if [ "${1:-}" = --range ]; then
	range=yes
	status=0
	for sha in $(git rev-list --no-merges "$2"); do
		check "$(git log -1 --format=%s "$sha")" || status=1
	done
	exit $status
fi

check "$(grep -v '^#' "$1" | sed -n '/./{p;q;}')"
