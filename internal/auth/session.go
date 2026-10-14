// Package auth owns the Blackboard session: how it is obtained (a human in
// visible Chrome), renewed (headless Chrome from the same profile), stored
// (macOS Keychain), and attached to requests (cookies + X-Blackboard-XSRF).
//
// Nothing outside this package sees cookie names, the BbRouter format, the
// Chrome profile path, or the keychain item. Callers hold a *Manager and ask
// it for a Session; they cannot build one by hand.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const routerCookie = "BbRouter"

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
func parseSession(cookies []*http.Cookie, savedAt time.Time) (Session, error) {
	router := routerValue(cookies)
	if router == "" {
		return Session{}, errors.New("no BbRouter cookie")
	}
	if v, err := url.QueryUnescape(router); err == nil {
		router = v
	}
	s := Session{cookies: cookies, savedAt: savedAt}
	for _, field := range strings.Split(router, ",") {
		key, val, _ := strings.Cut(strings.TrimSpace(field), ":")
		switch key {
		case "xsrf":
			s.xsrf = val
		case "expires":
			if sec, err := strconv.ParseInt(val, 10, 64); err == nil {
				s.expiresAt = time.Unix(sec, 0).UTC()
			}
		}
	}
	if s.xsrf == "" {
		return Session{}, errors.New("BbRouter cookie has no xsrf field")
	}
	return s, nil
}

func routerValue(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name == routerCookie {
			return c.Value
		}
	}
	return ""
}

// Apply attaches the session to an outbound request: Cookie header plus
// X-Blackboard-XSRF. Callers never set either header themselves.
func (s Session) Apply(r *http.Request) {
	// Built by hand: http.Request.AddCookie wraps values containing ',' in
	// quotes, and a raw BbRouter value is a comma-separated list.
	parts := make([]string, len(s.cookies))
	for i, c := range s.cookies {
		parts[i] = c.Name + "=" + c.Value
	}
	r.Header.Set("Cookie", strings.Join(parts, "; "))
	r.Header.Set("X-Blackboard-XSRF", s.xsrf)
}

// ExpiresAt is the server-declared expiry, or zero if BbRouter carried none.
// Advisory only: the server's 401 is the truth.
func (s Session) ExpiresAt() time.Time { return s.expiresAt }

// SavedAt is when the session was captured from Chrome.
func (s Session) SavedAt() time.Time { return s.savedAt }

func (s Session) same(o Session) bool { return routerValue(s.cookies) == routerValue(o.cookies) }

// stored is the keychain payload. Only name/value pairs are kept: every
// cookie is for blackboard.wm.edu, and go-keyring caps an item near 3 KB.
type stored struct {
	SavedAt time.Time   `json:"savedAt"`
	Cookies [][2]string `json:"cookies"`
}

func (s Session) marshal() ([]byte, error) {
	p := stored{SavedAt: s.savedAt}
	for _, c := range s.cookies {
		p.Cookies = append(p.Cookies, [2]string{c.Name, c.Value})
	}
	return json.Marshal(p)
}

func unmarshalSession(b []byte) (Session, error) {
	var p stored
	if err := json.Unmarshal(b, &p); err != nil {
		return Session{}, fmt.Errorf("stored session is not valid JSON: %w", err)
	}
	cookies := make([]*http.Cookie, 0, len(p.Cookies))
	for _, nv := range p.Cookies {
		cookies = append(cookies, &http.Cookie{Name: nv[0], Value: nv[1]})
	}
	return parseSession(cookies, p.SavedAt)
}
