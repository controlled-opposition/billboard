package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// ErrNeedsHuman means no automatic path can produce a session: nothing is
// stored and headless renewal failed (or there is no profile to renew from).
// The CLI maps it to exit code 4 with the instruction "ask the human to run
// `bb auth login`". Wrap it; never replace it.
var ErrNeedsHuman = errors.New("blackboard login required: a human must run `bb auth login`")

// RenewTimeout is the ceiling on one headless renewal attempt.
const RenewTimeout = 20 * time.Second

// Paths are the on-disk locations the Manager owns. Tests point them at a
// temp dir.
type Paths struct {
	Profile string // persistent Chrome user-data-dir (holds Entra "stay signed in" cookies)
	Lock    string // flock file serializing every writer of Profile + keychain
}

// PathsIn returns the Manager's paths inside dir.
func PathsIn(dir string) Paths {
	return Paths{Profile: filepath.Join(dir, "chrome-profile"), Lock: filepath.Join(dir, "auth.lock")}
}

// DefaultPaths returns ~/Library/Application Support/bb/{chrome-profile,auth.lock}.
func DefaultPaths() (Paths, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return PathsIn(filepath.Join(dir, "bb")), nil
}

// Keyring is the seam to the OS secret store: one item holding the session.
// Get returns ErrNoItem when the item is absent; Delete of an absent item
// is not an error.
type Keyring interface {
	Get() ([]byte, error)
	Set([]byte) error
	Delete() error
}

var ErrNoItem = errors.New("no stored session")

// BrowserRun is one Chrome session against the CAS login URL.
type BrowserRun struct {
	Profile  string
	Headless bool
	Timeout  time.Duration
}

// Browser drives Chrome until it lands on blackboard.wm.edu/ultra and
// returns the blackboard.wm.edu cookies. Headless runs return an error
// wrapping ErrNeedsHuman as soon as a page asks for credentials or MFA.
type Browser func(ctx context.Context, r BrowserRun) ([]*http.Cookie, error)

// Options configure a Manager. Nil Keyring and Browser select the macOS
// Keychain and system Chrome.
type Options struct {
	Paths   Paths
	Keyring Keyring
	Browser Browser
	NoRenew bool // BB_NO_RENEW=1: never launch headless Chrome
}

// Manager is the single owner of session state shared across bb processes:
// the keychain item and the Chrome profile. Reads are lock-free (keychain
// read). Every write (login, renew, logout) takes the flock at Paths.Lock,
// then re-reads the keychain before acting, so two processes that both see
// a 401 launch Chrome once: the second finds the fresh session the first
// saved and returns it.
type Manager struct {
	paths   Paths
	keyring Keyring
	browser Browser
	noRenew bool
}

func NewManager(o Options) *Manager {
	m := &Manager{paths: o.Paths, keyring: o.Keyring, browser: o.Browser, noRenew: o.NoRenew}
	if m.keyring == nil {
		m.keyring = systemKeyring{}
	}
	if m.browser == nil {
		m.browser = captureCookies
	}
	return m
}

// Current returns the stored session without touching the network or
// launching Chrome. No stored session is ErrNeedsHuman.
func (m *Manager) Current(ctx context.Context) (Session, error) {
	b, err := m.keyring.Get()
	if errors.Is(err, ErrNoItem) {
		return Session{}, fmt.Errorf("not logged in: %w", ErrNeedsHuman)
	}
	if err != nil {
		return Session{}, fmt.Errorf("read keychain: %w", err)
	}
	s, err := unmarshalSession(b)
	if err != nil {
		return Session{}, fmt.Errorf("%v: %w", err, ErrNeedsHuman)
	}
	return s, nil
}

// Renew replaces a session the server rejected. stale is the session that
// got the 401; if the keychain already holds a different session, Renew
// returns that without launching Chrome (another process renewed first).
// Otherwise it runs headless Chrome against the profile for at most
// RenewTimeout and saves the cookies. Every failure wraps ErrNeedsHuman.
func (m *Manager) Renew(ctx context.Context, stale Session) (Session, error) {
	if m.noRenew {
		return Session{}, fmt.Errorf("session expired and BB_NO_RENEW is set: %w", ErrNeedsHuman)
	}
	var fresh Session
	err := withLock(ctx, m.paths.Lock, RenewTimeout+5*time.Second, func() error {
		if cur, err := m.Current(ctx); err == nil && !cur.same(stale) {
			fresh = cur
			return nil
		}
		if !m.HasProfile() {
			return errors.New("session expired and there is no browser profile to renew from")
		}
		s, err := m.capture(ctx, BrowserRun{Profile: m.paths.Profile, Headless: true, Timeout: RenewTimeout})
		if err != nil {
			return fmt.Errorf("headless renewal failed: %w", err)
		}
		fresh = s
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNeedsHuman) {
			return Session{}, err
		}
		return Session{}, fmt.Errorf("%v: %w", err, ErrNeedsHuman)
	}
	return fresh, nil
}

// LoginOptions configure the human flow.
type LoginOptions struct {
	Timeout time.Duration // default 5m
	// Notify receives one human-readable line. The CLI points it at stderr.
	// Never a prompt.
	Notify func(string)
}

// Login opens visible Chrome on the CAS login URL with the persistent
// profile, waits for blackboard.wm.edu/ultra*, and saves the cookies.
// Idempotent: if the profile is still signed in, Chrome lands on /ultra
// immediately and Login returns without human input.
func (m *Manager) Login(ctx context.Context, o LoginOptions) (Session, error) {
	if o.Timeout == 0 {
		o.Timeout = 5 * time.Minute
	}
	if err := os.MkdirAll(m.paths.Profile, 0o700); err != nil {
		return Session{}, err
	}
	var s Session
	err := withLock(ctx, m.paths.Lock, time.Minute, func() error {
		if o.Notify != nil {
			o.Notify(fmt.Sprintf("complete the NetID + MFA login in the Chrome window (timeout %s)", o.Timeout))
		}
		var err error
		s, err = m.capture(ctx, BrowserRun{Profile: m.paths.Profile, Timeout: o.Timeout})
		return err
	})
	return s, err
}

// capture runs the browser, parses the cookies, and saves the session.
// Callers hold the lock.
func (m *Manager) capture(ctx context.Context, r BrowserRun) (Session, error) {
	cookies, err := m.browser(ctx, r)
	if err != nil {
		return Session{}, err
	}
	s, err := parseSession(cookies, time.Now().UTC())
	if err != nil {
		return Session{}, fmt.Errorf("landed on Blackboard but %w", err)
	}
	b, err := s.marshal()
	if err != nil {
		return Session{}, err
	}
	if err := m.keyring.Set(b); err != nil {
		return Session{}, fmt.Errorf("save session to keychain: %w", err)
	}
	return s, nil
}

// Logout deletes the keychain item and removes the Chrome profile directory.
// Idempotent: both steps succeed when already absent. Takes the flock.
func (m *Manager) Logout(ctx context.Context) error {
	return withLock(ctx, m.paths.Lock, time.Minute, func() error {
		if err := m.keyring.Delete(); err != nil {
			return fmt.Errorf("delete keychain item: %w", err)
		}
		return os.RemoveAll(m.paths.Profile)
	})
}

// HasProfile reports whether headless renewal has anything to work with.
func (m *Manager) HasProfile() bool {
	fi, err := os.Stat(m.paths.Profile)
	return err == nil && fi.IsDir()
}

// systemKeyring is the macOS Keychain item service "bb", account
// "blackboard.wm.edu".
type systemKeyring struct{}

const (
	keyringService = "bb"
	keyringUser    = "blackboard.wm.edu"
)

func (systemKeyring) Get() ([]byte, error) {
	s, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNoItem
	}
	return []byte(s), err
}

func (systemKeyring) Set(b []byte) error { return keyring.Set(keyringService, keyringUser, string(b)) }

func (systemKeyring) Delete() error {
	if err := keyring.Delete(keyringService, keyringUser); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}
