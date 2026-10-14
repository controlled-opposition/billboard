// Package cli is the thin shell: flags -> bb calls -> output.Print. Command
// files contain no HTTP, no wire types, and no rendering logic.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

// env is built once per process and handed to every command constructor.
// The client is lazy so `bb --help`, `bb auth logout`, and usage errors
// never touch the keychain.
type env struct {
	streams output.Streams
	auth    *auth.Manager
	client  func() *bb.Client // memoized; read-only
}

// Run parses args, executes, prints any error to stderr as one line
// ("bb: <message>", plus a hint line for exit 4/5), and returns the code.
// It never reads stdin and never prompts.
func Run(args []string) ExitCode { panic("not implemented") }

func newRoot(e *env) *cobra.Command {
	root := &cobra.Command{
		Use:           "bb <command> <subcommand> [flags]",
		Short:         "Blackboard Ultra (W&M) from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newAuthCmd(e),
		newCourseCmd(e),
		newDueCmd(e),
		newContentCmd(e),
		// later: newGradeCmd(e), newAnnouncementCmd(e), newAPICmd(e)
	)
	return root
}

// coursesAndResolve fetches the course list once and resolves every arg
// against it. The shared preamble for any command taking course args.
func coursesAndResolve(ctx context.Context, c *bb.Client, args []string) (all []bb.Course, picked []bb.Course, err error) {
	panic("not implemented")
}
