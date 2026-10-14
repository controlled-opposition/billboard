package auth

import (
	"context"
	"net/http"
	"time"
)

// casLoginURL enters the CAS -> Entra SAML -> MFA chain and redirects to /ultra.
const casLoginURL = "https://blackboard.wm.edu/webapps/bb-auth-provider-cas-BB571910f3bd595/execute/casLogin?cmd=login&authProviderId=_142_1&redirectUrl=https%3A%2F%2Fblackboard.wm.edu%2Fultra"

// captureCookies is the one chromedp entry point. headless=true rewrites the
// UA via Browser.getVersion + Emulation.setUserAgentOverride. It navigates to
// casLoginURL, polls the main-frame URL until host == blackboard.wm.edu and
// path has prefix /ultra, then returns Storage.getCookies filtered to that
// domain (URL-based lookup filters by path and misses BbRouter).
//
// TODO: port navigateAndWait/cookiesFor from the prototype; pin chromedp v0.21.0.
func captureCookies(ctx context.Context, chrome, profile string, headless bool, timeout time.Duration) ([]*http.Cookie, error) {
	panic("not implemented")
}

// withLock runs fn holding an exclusive flock on path (blocking, ctx-bounded).
func withLock(ctx context.Context, path string, fn func() error) error {
	panic("not implemented")
}
