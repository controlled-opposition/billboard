package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/output"
)

func newAuthCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Log in to Blackboard, check, or log out"}
	cmd.AddCommand(newAuthLoginCmd(e), newAuthStatusCmd(e), newAuthLogoutCmd(e))
	return cmd
}

func newAuthLoginCmd(e *env) *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in through NetID + MFA in a Chrome window (human only)",
		Long: `Open Chrome on the W&M login page and wait until a human finishes NetID
and MFA. The session goes to the macOS Keychain; the Chrome profile is kept
so later commands can renew the session without a human.

An agent may start this for a human at the machine, but cannot complete it.
If the profile is still signed in, it returns without any input.`,
		Args: positional(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := e.auth.Login(cmd.Context(), auth.LoginOptions{
				Timeout: timeout,
				Notify:  func(s string) { fmt.Fprintf(e.streams.Err, "bb: %s\n", s) },
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(e.streams.Err, "bb: logged in")
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "how long to wait for the human")
	return cmd
}

// AuthStatus is what `bb auth status` reports. Status makes one request
// (users/me) through the normal client, so a stale session is renewed
// headlessly if possible: "logged in" means "the next command will work".
type AuthStatus struct {
	LoggedIn  bool
	User      string // NetID
	Name      string
	ExpiresAt time.Time
	CanRenew  bool // Chrome profile present
}

var authStatusSchema = output.Schema[AuthStatus]{
	Fields: []output.Field[AuthStatus]{
		{Name: "loggedIn", Doc: "session works right now", Value: func(s AuthStatus) any { return s.LoggedIn }},
		{Name: "user", Doc: "NetID", Value: func(s AuthStatus) any { return s.User }},
		{Name: "name", Doc: "display name", Value: func(s AuthStatus) any { return s.Name }},
		{Name: "expiresAt", Doc: "session cookie expiry (advisory)", Value: func(s AuthStatus) any { return s.ExpiresAt }},
		{Name: "canRenew", Doc: "headless renewal possible", Value: func(s AuthStatus) any { return s.CanRenew }},
	},
	Default: []string{"loggedIn", "user", "expiresAt"},
}

func newAuthStatusCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check that the stored session works",
		Long: `Make one request as the stored session (renewing it headlessly if needed)
and print one status row. Exits 0 when logged in and 4 when a human must
run ` + "`bb auth login`" + `; the row is printed either way.`,
		Args: positional(cobra.NoArgs),
	}
	o := output.Bind(cmd, authStatusSchema)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		st := AuthStatus{CanRenew: e.auth.HasProfile()}
		me, err := e.client().Me(cmd.Context())
		if err == nil {
			st.LoggedIn, st.User, st.Name = true, me.UserName, me.Name
			if s, err := e.auth.Current(cmd.Context()); err == nil {
				st.ExpiresAt = s.ExpiresAt()
			}
		}
		return errors.Join(output.Print(e.streams, authStatusSchema, o, []AuthStatus{st}), err)
	}
	return cmd
}

func newAuthLogoutCmd(e *env) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored session and the Chrome profile",
		Long:  "Delete the keychain item and the Chrome profile. Succeeds when already logged out.",
		Args:  positional(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := e.auth.Logout(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(e.streams.Err, "bb: logged out")
			return nil
		},
	}
}
