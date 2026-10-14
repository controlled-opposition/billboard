package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/output"
)

func newAuthCmd(e *env) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Log in to Blackboard, check, or log out"}
	cmd.AddCommand(newAuthLoginCmd(e), newAuthStatusCmd(e), newAuthLogoutCmd(e))
	return cmd
}

// bb auth login [--timeout 5m]
// Human-only: opens Chrome. Under a non-TTY it still runs (an agent may
// launch it for a human sitting at the machine) but prints one stderr line
// saying a human must finish the login in the browser window.
func newAuthLoginCmd(e *env) *cobra.Command { panic("not implemented") }

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

// bb auth status: exit 0 when logged in, exit 4 when not (and still prints
// the status row), so `bb auth status >/dev/null || ask-human` works.
func newAuthStatusCmd(e *env) *cobra.Command { panic("not implemented") }

// bb auth logout: clears keychain + Chrome profile. Idempotent; exit 0 when
// already logged out.
func newAuthLogoutCmd(e *env) *cobra.Command { panic("not implemented") }
