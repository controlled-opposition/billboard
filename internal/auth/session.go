// Package auth owns the Blackboard session: how it is obtained (a human in
// visible Chrome), renewed (headless Chrome from the same profile), stored
// (macOS Keychain), and attached to requests (cookies + X-Blackboard-XSRF).
//
// Nothing outside this package sees cookie names, the BbRouter format, the
// Chrome profile path, or the keychain item. Callers hold a *Manager and ask
// it for a Session; they cannot build one by hand.
package auth

import (
	"net/http"
	"time"
)

// Session is a parsed, usable Blackboard session. The zero value is never
// returned by this package: every Session has a BbRouter cookie and the xsrf
// token parsed out of it. Fields are unexported so no caller can construct a
// Session that lacks the XSRF token.
type Session struct {
	cookies   []*http.Cookie // every blackboard.wm.edu cookie Chrome held, BbRouter included
	xsrf      string         // the xsrf:<uuid> field inside BbRouter
	expiresAt time.Time      // from BbRouter's expires: field; zero if absent
	savedAt   time.Time
}

// parseSession is the only constructor. It validates the BbRouter invariant
// at the boundary (Chrome cookie jar or keychain JSON) so nothing downstream
// re-checks it.
//
// TODO: url.QueryUnescape BbRouter, split on ',', take "xsrf:" and "expires:".
func parseSession(cookies []*http.Cookie, savedAt time.Time) (Session, error) {
	panic("not implemented")
}

// Apply attaches the session to an outbound request: Cookie header plus
// X-Blackboard-XSRF. Callers never set either header themselves.
func (s Session) Apply(r *http.Request) { panic("not implemented") }

// ExpiresAt is the server-declared expiry, or zero if BbRouter carried none.
// Advisory only: the server's 401 is the truth.
func (s Session) ExpiresAt() time.Time { return s.expiresAt }

// SavedAt is when the session was captured from Chrome.
func (s Session) SavedAt() time.Time { return s.savedAt }

// marshal/unmarshal is the keychain payload format; private so the format has
// one owner.
func (s Session) marshal() ([]byte, error)       { panic("not implemented") }
func unmarshalSession(b []byte) (Session, error) { panic("not implemented") }
