package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

// casLoginURL enters the CAS -> Entra SAML -> MFA chain and redirects to /ultra.
const casLoginURL = "https://blackboard.wm.edu/webapps/bb-auth-provider-cas-BB571910f3bd595/execute/casLogin?cmd=login&authProviderId=_142_1&redirectUrl=https%3A%2F%2Fblackboard.wm.edu%2Fultra"

const blackboardHost = "blackboard.wm.edu"

// humanNeededJS is true when the page shows anything only a person can
// answer: Entra's email, password, or code fields, the Authenticator
// number-match prompt, the account picker, or "Stay signed in?".
const humanNeededJS = `[...document.querySelectorAll(
	'input[name=loginfmt],input[type=password],input[name=otc],#idRichContext_DisplaySign,#tilesHolder,#KmsiCheckboxField'
)].some(e => e.offsetParent !== null)`

// captureCookies is the production Browser. Headless runs rewrite the UA
// (cas.wm.edu answers 403 to "HeadlessChrome") and give up as soon as a
// page wants a human. Cookies come from the browser-level Storage.getCookies
// filtered by domain, because a URL-based lookup filters by path and misses
// BbRouter.
func captureCookies(ctx context.Context, r BrowserRun) ([]*http.Cookie, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.UserDataDir(r.Profile))
	if !r.Headless {
		opts = append(opts, chromedp.VisibleWindow)
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	bctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	// The first Run starts the browser; a deadline on it would kill Chrome,
	// so the timeout below is a wall clock, not a context deadline.
	if err := chromedp.Do(bctx); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}
	if r.Headless {
		v, err := chromedp.CallBrowser(bctx, cdpbrowser.GetVersion, cdp.Empty{})
		if err == nil {
			ua := strings.Replace(v.UserAgent, "HeadlessChrome", "Chrome", 1)
			_, err = chromedp.Call(bctx, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{UserAgent: ua})
		}
		if err != nil {
			return nil, fmt.Errorf("override user agent: %w", err)
		}
	}
	if _, err := chromedp.Call(bctx, page.Navigate, page.NavigateParams{URL: casLoginURL}); err != nil {
		return nil, fmt.Errorf("open login page: %w", err)
	}

	deadline := time.Now().Add(r.Timeout)
	var last string
	for {
		if loc, err := chromedp.Run(bctx, chromedp.Location()); err == nil {
			last = loc
			if u, err := url.Parse(loc); err == nil && u.Host == blackboardHost && strings.HasPrefix(u.Path, "/ultra") {
				break
			}
		}
		if r.Headless {
			if ask, err := chromedp.Run(bctx, chromedp.Evaluate[bool](humanNeededJS)); err == nil && ask {
				return nil, fmt.Errorf("sign-in page at %s asks for credentials or MFA: %w", hostOf(last), ErrNeedsHuman)
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("did not reach Blackboard within %s (last page: %s)", r.Timeout, hostOf(last))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	// Blackboard may still be setting cookies on the /ultra response.
	time.Sleep(200 * time.Millisecond)

	res, err := chromedp.CallBrowser(bctx, storage.GetCookies, storage.GetCookiesParams{})
	if err != nil {
		return nil, fmt.Errorf("read cookies: %w", err)
	}
	var out []*http.Cookie
	for _, c := range res.Cookies {
		if d := strings.TrimPrefix(c.Domain, "."); d == blackboardHost || strings.HasSuffix(blackboardHost, "."+d) {
			out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
	return out, nil
}

// hostOf names a page without its query string, which can carry tokens.
func hostOf(loc string) string {
	if u, err := url.Parse(loc); err == nil && u.Host != "" {
		return u.Host + u.Path
	}
	return "unknown"
}

// withLock runs fn holding an exclusive flock on path, waiting at most wait
// for another bb process to release it.
func withLock(ctx context.Context, path string, wait time.Duration, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(wait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("lock %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("another bb process held %s for over %s", path, wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
