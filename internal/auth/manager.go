package auth

import (
	"context"
	"errors"
	"time"
)

// ErrNeedsHuman means no automatic path can produce a session: nothing is
// stored and headless renewal failed (or there is no profile to renew from).
// The CLI maps it to exit code 4 with the instruction "ask the human to run
// `bb auth login`". Wrap it; never replace it.
var ErrNeedsHuman = errors.New("blackboard login required: a human must run `bb auth login`")

// Paths are the on-disk locations the Manager owns. Derived once from
// os.UserConfigDir; tests point them at a temp dir.
type Paths struct {
	Profile string // persistent Chrome user-data-dir (holds Entra "stay signed in" cookies)
	Lock    string // flock file serializing every writer of Profile + keychain
}

// DefaultPaths returns ~/Library/Application Support/bb/{chrome-profile,auth.lock}.
func DefaultPaths() (Paths, error) { panic("not implemented") }

// Manager is the single owner of session state shared across bb processes:
// the keychain item and the Chrome profile. An agent may run several `bb`
// processes at once, so those two resources are shared between processes.
// Reads are lock-free (keychain read). Every write (login, renew, logout)
// takes the flock at Paths.Lock, then re-reads the keychain before acting, so
// two processes that both see a 401 launch Chrome once: the second finds the
// fresh session the first saved and returns it.
type Manager struct {
	paths   Paths
	keyring keyring
	chrome  string // Chrome executable path
}

// keyring is the seam to macOS Keychain (zalando/go-keyring in production, a
// map in tests). One item: service "bb", account "blackboard.wm.edu".
type keyring interface {
	Get() ([]byte, error) // returns errNoItem when absent
	Set([]byte) error
	Delete() error // idempotent: absent item is not an error
}

func NewManager(p Paths) *Manager { panic("not implemented") }

// Current returns the stored session without touching the network. It
// returns ErrNeedsHuman only when the keychain is empty AND the profile
// does not exist; otherwise an empty keychain triggers Renew.
func (m *Manager) Current(ctx context.Context) (Session, error) { panic("not implemented") }

// Renew replaces a session the server rejected. stale is the session that
// got the 401; if the keychain already holds a different (newer) session,
// Renew returns that without launching Chrome (another process renewed
// first). Otherwise it runs headless Chrome against the profile with the
// HeadlessChrome UA rewritten (cas.wm.edu 403s it), waits up to ~30s for
// blackboard.wm.edu/ultra*, and saves the cookies.
//
// Any failure to land on /ultra (MFA prompt, Entra password page, timeout)
// returns an error wrapping ErrNeedsHuman. Unproven against the live chain.
func (m *Manager) Renew(ctx context.Context, stale Session) (Session, error) {
	panic("not implemented")
}

// LoginOptions configure the human flow.
type LoginOptions struct {
	Timeout time.Duration // default 5m
	// Notify receives one human-readable line ("complete NetID + MFA in the
	// Chrome window"). The CLI points it at stderr. Never a prompt.
	Notify func(string)
}

// Login opens visible Chrome on the CAS login URL with the persistent
// profile, waits for blackboard.wm.edu/ultra*, reads cookies via the
// browser-level Storage.getCookies filtered by domain, and saves them.
// Idempotent: if the profile is still signed in, Chrome lands on /ultra
// immediately and Login returns without human input.
func (m *Manager) Login(ctx context.Context, o LoginOptions) (Session, error) {
	panic("not implemented")
}

// Logout deletes the keychain item and removes the Chrome profile directory.
// Idempotent: both steps succeed when already absent. Takes the flock.
func (m *Manager) Logout() error { panic("not implemented") }

// HasProfile reports whether headless renewal has anything to work with.
func (m *Manager) HasProfile() bool { panic("not implemented") }
