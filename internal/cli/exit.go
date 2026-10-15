package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/bb"
)

// ExitCode is the agent's control channel. Every command's --help lists
// exitCodes, the same table classify returns from, so they cannot drift.
type ExitCode int

const (
	ExitOK          ExitCode = 0
	ExitError       ExitCode = 1
	ExitUsage       ExitCode = 2
	ExitForbidden   ExitCode = 3
	ExitNeedsHuman  ExitCode = 4
	ExitNotFound    ExitCode = 5
	ExitPartial     ExitCode = 6
	ExitUnavailable ExitCode = 7
)

var exitCodes = []struct {
	Code ExitCode
	Doc  string
}{
	{ExitOK, "success (including empty results)"},
	{ExitError, "error; message on stderr"},
	{ExitUsage, "invalid usage; nothing was sent to Blackboard"},
	{ExitForbidden, "Blackboard denied access to this resource (HTTP 403); logging in again will not help"},
	{ExitNeedsHuman, "login required: ask the human to run `bb auth login`, then retry"},
	{ExitNotFound, "no match, or several matches (candidates on stderr), for a course/content argument; or HTTP 404"},
	{ExitPartial, "partial failure; see per-item status on stdout, and the failed courses or items on stderr"},
	{ExitUnavailable, "Blackboard unavailable (5xx, 429, network); retry later"},
}

func exitCodesHelp() string {
	var b strings.Builder
	b.WriteString("EXIT CODES\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, e := range exitCodes {
		fmt.Fprintf(tw, "  %d\t%s\n", e.Code, e.Doc)
	}
	tw.Flush()
	return b.String()
}

// errPartial is returned by commands that already printed per-item results.
var errPartial = errors.New("some items failed")

// usageError marks flag/arg problems detected before any request.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// classify is the only place errors become exit codes.
func classify(err error) ExitCode {
	var ue usageError
	var ae *bb.APIError
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, auth.ErrNeedsHuman):
		return ExitNeedsHuman
	case errors.As(err, &ue):
		return ExitUsage
	case errors.Is(err, errPartial), bb.IsPartial(err):
		return ExitPartial
	case errors.As(err, &ae):
		switch ae.Kind {
		case bb.KindNotFound, bb.KindAmbiguous:
			return ExitNotFound
		case bb.KindForbidden:
			return ExitForbidden
		case bb.KindUnavailable:
			return ExitUnavailable
		}
	}
	return ExitError
}
