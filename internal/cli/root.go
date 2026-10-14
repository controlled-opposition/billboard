// Package cli is the thin shell: flags -> bb calls -> output.Print. Command
// files contain no HTTP, no wire types, and no rendering logic.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/bb"
	"github.com/controlled-opposition/billboard/internal/output"
)

// Env is everything Execute takes from the process. Tests fill it with
// buffers, an httptest BaseURL, a temp ConfigDir, and fake Keyring/Browser.
type Env struct {
	Stdout, Stderr io.Writer
	StdoutTTY      bool
	Width          int
	Getenv         func(string) string
	Now            func() time.Time
	BaseURL        string       // "" = https://blackboard.wm.edu
	ConfigDir      string       // "" = <user config dir>/bb
	Keyring        auth.Keyring // nil = macOS Keychain
	Browser        auth.Browser // nil = system Chrome
}

// OSEnv is the real process environment.
func OSEnv() Env {
	t := term.FromEnv()
	w, _, _ := t.Size()
	return Env{
		Stdout:    t.Out(),
		Stderr:    t.ErrOut(),
		StdoutTTY: t.IsTerminalOutput(),
		Width:     w,
		Getenv:    os.Getenv,
		Now:       time.Now,
	}
}

// env is built once per process and handed to every command constructor.
// The client is lazy so `bb --help`, `bb auth logout`, and usage errors
// never touch the keychain.
type env struct {
	streams     output.Streams
	auth        *auth.Manager
	clientOpts  []bb.Option
	maxRequests int
	c           *bb.Client
}

func (e *env) client() *bb.Client {
	if e.c == nil {
		e.c = bb.New(e.auth, append(e.clientOpts, bb.WithMaxRequests(e.maxRequests))...)
	}
	return e.c
}

func newEnv(x Env) (*env, error) {
	if x.Getenv == nil {
		x.Getenv = os.Getenv
	}
	if x.Now == nil {
		x.Now = time.Now
	}
	paths, err := auth.DefaultPaths()
	if x.ConfigDir != "" {
		paths, err = auth.PathsIn(x.ConfigDir), nil
	}
	if err != nil {
		return nil, err
	}
	e := &env{
		streams: output.Streams{Out: x.Stdout, Err: x.Stderr, OutTTY: x.StdoutTTY, Width: x.Width},
		auth: auth.NewManager(auth.Options{
			Paths:   paths,
			Keyring: x.Keyring,
			Browser: x.Browser,
			NoRenew: x.Getenv("BB_NO_RENEW") == "1",
		}),
		clientOpts: []bb.Option{bb.WithClock(x.Now)},
	}
	if x.BaseURL != "" {
		u, err := url.Parse(x.BaseURL)
		if err != nil {
			return nil, err
		}
		e.clientOpts = append(e.clientOpts, bb.WithBaseURL(u))
	}
	return e, nil
}

// Execute parses args, runs the command, prints any error to stderr as
// "bb: <message>" (plus candidates for an ambiguous argument and a hint for
// exit 4), and returns the exit code. It never reads stdin and never prompts.
func Execute(ctx context.Context, x Env, args []string) int {
	e, err := newEnv(x)
	if err != nil {
		fmt.Fprintf(x.Stderr, "bb: %v\n", err)
		return int(ExitError)
	}
	root := newRoot(e)
	root.SetArgs(args)
	root.SetOut(x.Stdout)
	root.SetErr(x.Stderr)
	err = root.ExecuteContext(ctx)
	// cobra reports an unknown subcommand as a plain error.
	if err != nil && strings.HasPrefix(err.Error(), "unknown command") {
		err = usageError{err.Error()}
	}
	code := classify(err)
	if err != nil {
		report(x.Stderr, err, code)
	}
	return int(code)
}

func report(w io.Writer, err error, code ExitCode) {
	fmt.Fprintf(w, "bb: %v\n", err)
	var ae *bb.APIError
	if errors.As(err, &ae) && len(ae.Candidates) > 0 {
		for _, c := range ae.Candidates {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", c.Code, c.ID, c.Name, c.Term)
		}
	}
	if code == ExitNeedsHuman {
		fmt.Fprintln(w, "bb: ask the human to run `bb auth login`, then retry")
	}
}

const rootLong = `Read W&M Blackboard Ultra from the command line.

Output is tab-separated with no header when stdout is not a terminal, and
an aligned table when it is. Commands that list things take --json FIELDS
(see each command's JSON FIELDS), --jq EXPR, and --template TMPL.

bb never prompts. Branch on the exit code: 4 means a human must run
` + "`bb auth login`" + ` (NetID + MFA in Chrome); every other command reuses that
session and renews it headlessly when it can (BB_NO_RENEW=1 disables that).`

func newRoot(e *env) *cobra.Command {
	root := &cobra.Command{
		Use:           "bb <command> <subcommand> [flags]",
		Short:         "Blackboard Ultra (W&M) from the command line",
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err.Error()} })
	root.SetHelpTemplate(root.HelpTemplate() + "\n" + exitCodesHelp())
	root.PersistentFlags().IntVar(&e.maxRequests, "max-requests", bb.DefaultMaxRequests, "stop after this many requests to Blackboard")
	root.AddCommand(
		newAuthCmd(e),
		newCourseCmd(e),
		newAPICmd(e),
		newDueCmd(e),
		newContentCmd(e),
	)
	return root
}

// positional wraps a cobra.PositionalArgs so its failures are usage errors.
func positional(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if err := v(c, args); err != nil {
			return usageError{err.Error()}
		}
		return nil
	}
}

func notImplemented(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE:  func(*cobra.Command, []string) error { return errors.New("not implemented yet") },
	}
}

// coursesAndResolve fetches the course list once and resolves every arg
// against it. The shared preamble for any command taking course args.
func coursesAndResolve(ctx context.Context, c *bb.Client, args []string) (all []bb.Course, picked []bb.Course, err error) {
	all, err = c.Courses(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range args {
		course, err := bb.ResolveCourse(all, a)
		if err != nil {
			return nil, nil, err
		}
		picked = append(picked, course)
	}
	return all, picked, nil
}
