package cli

import "errors"

// ExitCode is the agent's control channel. Documented in `bb --help` (the
// EXIT CODES section is generated from exitDocs, the same table the switch
// below uses, so they cannot drift).
type ExitCode int

const (
	ExitOK          ExitCode = 0
	ExitError       ExitCode = 1 // anything unclassified; message on stderr
	ExitUsage       ExitCode = 2 // bad flags/args; nothing was sent to Blackboard
	ExitNeedsHuman  ExitCode = 4 // ask the human to run `bb auth login` (gh's auth code)
	ExitNotFound    ExitCode = 5 // course/content arg matched nothing, or matched several (candidates on stderr)
	ExitPartial     ExitCode = 6 // some items succeeded, some failed (per-item status on stdout)
	ExitUnavailable ExitCode = 7 // Blackboard 5xx/429/network; retrying later may work
	// 403 on a specific resource is ExitError with the server message: it is
	// not retryable and not fixable by login.
)

var exitDocs = []struct {
	Code ExitCode
	Doc  string
}{
	{ExitOK, "success (including empty results)"},
	{ExitError, "error"},
	{ExitUsage, "invalid usage"},
	{ExitNeedsHuman, "login required: a human must run `bb auth login`"},
	{ExitNotFound, "no match or ambiguous match for a course/content argument"},
	{ExitPartial, "partial failure; see per-item status"},
	{ExitUnavailable, "Blackboard unavailable; retry later"},
}

// errPartial is returned by commands that already printed per-item results.
var errPartial = errors.New("some items failed")

// usageError marks flag/arg problems detected before any request.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// classify is the only place errors become exit codes: errors.Is
// auth.ErrNeedsHuman -> 4; *bb.APIError by Kind; usageError and cobra flag
// errors -> 2; errPartial -> 6.
func classify(err error) ExitCode { panic("not implemented") }
