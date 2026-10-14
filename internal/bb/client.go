// Package bb is the deep module: everything bb knows about Blackboard Learn
// Ultra at W&M lives here. Callers get domain types (Course, DueItem,
// ContentNode, Attachment) with HTML-unescaped strings and time.Time values.
// Wire structs, endpoint paths, paging, the Ultra "ultraDocumentBody" fold,
// data-bbfile parsing, and 401-then-renew are all unexported.
//
// Each domain file (course.go, due.go, content.go, download.go) owns its
// endpoints, its unexported wire struct, its pure parse function, and its
// public methods, so knowledge is grouped by noun, not by pipeline stage.
package bb

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/controlled-opposition/billboard/internal/auth"
)

// Sessions is what the client needs from auth. *auth.Manager implements it.
type Sessions interface {
	Current(ctx context.Context) (auth.Session, error)
	Renew(ctx context.Context, stale auth.Session) (auth.Session, error)
}

// DefaultMaxRequests is the per-process request cap.
const DefaultMaxRequests = 300

// Client talks to one Blackboard host. It is read-only unless built with
// AllowWrites; the check lives in do(), so no write can sneak through a new
// method or through Raw.
type Client struct {
	base     *url.URL
	http     *http.Client
	sessions Sessions
	writes   bool
	budget   *budget
	now      func() time.Time
}

type Option func(*Client)

// WithBaseURL points the client at another host, such as an httptest server.
func WithBaseURL(u *url.URL) Option { return func(c *Client) { c.base = u } }

// AllowWrites enables non-GET methods. Only commands with an explicit write
// flag pass it.
func AllowWrites() Option { return func(c *Client) { c.writes = true } }

// WithMaxRequests caps the requests this client sends.
func WithMaxRequests(n int) Option { return func(c *Client) { c.budget.max = n } }

// WithClock replaces time.Now for deciding which courses are current.
func WithClock(now func() time.Time) Option { return func(c *Client) { c.now = now } }

// New defaults to https://blackboard.wm.edu, 20s timeout, read-only,
// DefaultMaxRequests, one request per 250ms.
func New(s Sessions, opts ...Option) *Client {
	c := &Client{
		base:     &url.URL{Scheme: "https", Host: "blackboard.wm.edu"},
		http:     &http.Client{Timeout: 20 * time.Second},
		sessions: s,
		budget:   &budget{interval: 250 * time.Millisecond, max: DefaultMaxRequests},
		now:      time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// resolve turns a server-relative reference ("/learn/api/...?x=y", or a
// paging.nextPage value) into a URL on the client's host. Anything that
// would leave the host is refused, because do() attaches the session cookie.
func (c *Client) resolve(ref string) (*url.URL, error) {
	r, err := url.Parse(ref)
	if err != nil || !strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "//") || r.Host != "" {
		return nil, fmt.Errorf("path must start with / and stay on %s: %q", c.base.Host, ref)
	}
	return c.base.ResolveReference(r), nil
}

// do is the single HTTP path. Policy, in order:
//  1. refuse non-GET unless c.writes (ErrWriteRefused);
//  2. take one request from the budget (pacing, per-process cap);
//  3. attach the current session;
//  4. on 401: Renew(stale) once, retry once; second 401 -> error wrapping
//     auth.ErrNeedsHuman;
//  5. non-2xx -> *APIError with Kind from the status and Blackboard's message.
//
// The caller closes the returned body.
func (c *Client) do(ctx context.Context, method string, u *url.URL) (*http.Response, error) {
	if method != http.MethodGet && method != http.MethodHead && !c.writes {
		return nil, ErrWriteRefused
	}
	sess, err := c.sessions.Current(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, method, u, sess)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if sess, err = c.sessions.Renew(ctx, sess); err != nil {
			return nil, err
		}
		if resp, err = c.send(ctx, method, u, sess); err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			return nil, fmt.Errorf("Blackboard rejected the renewed session: %w", auth.ErrNeedsHuman)
		}
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, statusError(resp)
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, method string, u *url.URL, sess auth.Session) (*http.Response, error) {
	if err := c.budget.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	sess.Apply(req)
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &APIError{Kind: KindUnavailable, Message: err.Error()}
	}
	return resp, nil
}

func statusError(resp *http.Response) *APIError {
	e := &APIError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	var body struct {
		Message string `json:"message"`
	}
	if b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10)); err == nil && json.Unmarshal(b, &body) == nil && body.Message != "" {
		e.Message = body.Message
	}
	switch s := resp.StatusCode; {
	case s == http.StatusForbidden:
		e.Kind = KindForbidden
	case s == http.StatusNotFound:
		e.Kind = KindNotFound
	case s == http.StatusTooManyRequests || s >= 500:
		e.Kind = KindUnavailable
	}
	return e
}

// getJSON GETs ref (relative to base) and decodes into out.
func (c *Client) getJSON(ctx context.Context, ref string, out any) error {
	u, err := c.resolve(ref)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodGet, u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", u.Path, err)
	}
	return nil
}

// getPaged follows paging.nextPage until exhausted, appending results.
func getPaged[W any](ctx context.Context, c *Client, ref string) ([]W, error) {
	var all []W
	for ref != "" {
		var page struct {
			Results []W `json:"results"`
			Paging  struct {
				NextPage string `json:"nextPage"`
			} `json:"paging"`
		}
		if err := c.getJSON(ctx, ref, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		ref = page.Paging.NextPage
	}
	return all, nil
}

// Raw is the escape hatch for `bb api <path>`: same session, renewal, write
// gate, and error mapping as every other call; the response body is returned
// unparsed. path must be absolute on the Blackboard host ("/learn/api/...").
// It is the only exported method that exposes wire JSON, and only as bytes.
func (c *Client) Raw(ctx context.Context, method, path string) (io.ReadCloser, error) {
	u, err := c.resolve(path)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, method, u)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Me returns the logged-in user. Used by `bb auth status` to prove the
// session works and by Courses (memberships are per user id).
func (c *Client) Me(ctx context.Context) (User, error) {
	var w struct {
		ID       string `json:"id"`
		UserName string `json:"userName"`
		Name     struct {
			Given  string `json:"given"`
			Family string `json:"family"`
		} `json:"name"`
	}
	if err := c.getJSON(ctx, "/learn/api/public/v1/users/me", &w); err != nil {
		return User{}, err
	}
	return User{
		ID:       UserID(w.ID),
		UserName: w.UserName,
		Name:     html.UnescapeString(strings.TrimSpace(w.Name.Given + " " + w.Name.Family)),
	}, nil
}

type User struct {
	ID       UserID
	UserName string // NetID
	Name     string // given + family, unescaped
}
