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
	"io"
	"net/http"
	"net/url"

	"github.com/controlled-opposition/billboard/internal/auth"
)

// Sessions is what the client needs from auth. *auth.Manager implements it;
// tests pass a fixed session against an httptest server.
type Sessions interface {
	Current(ctx context.Context) (auth.Session, error)
	Renew(ctx context.Context, stale auth.Session) (auth.Session, error)
}

// Client talks to one Blackboard host. It is read-only unless built with
// AllowWrites; the check lives in do(), so no write can sneak through a new
// method or through Raw.
type Client struct {
	base     *url.URL
	http     *http.Client
	sessions Sessions
	writes   bool
	budget   *budget // per-process request counter + pacing; see budget.go
}

type Option func(*Client)

// WithBaseURL points the client at an httptest server.
func WithBaseURL(u *url.URL) Option { panic("not implemented") }

// AllowWrites enables non-GET methods. Only commands with an explicit write
// flag (future `bb api -X POST`, submission) pass it.
func AllowWrites() Option { panic("not implemented") }

// New defaults to https://blackboard.wm.edu, 20s timeout, read-only.
func New(s Sessions, opts ...Option) *Client { panic("not implemented") }

// do is the single HTTP path. Policy, in order:
//  1. refuse non-GET unless c.writes (ErrWriteRefused);
//  2. take one request from the budget (pacing, ~4 req/s);
//  3. attach the current session;
//  4. on 401: Renew(stale) once, retry once; second 401 -> error wrapping
//     auth.ErrNeedsHuman;
//  5. 403 -> *APIError{Kind: KindForbidden}; 404 -> KindNotFound;
//     5xx/429 -> KindUnavailable; body message is parsed from {"status","message"}.
//
// Callers inside the package use getJSON / getPaged; nothing outside calls do.
func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	panic("not implemented")
}

// getJSON GETs path (relative to base) and decodes into out.
func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	panic("not implemented")
}

// getPaged follows paging.nextPage until exhausted, appending results.
func getPaged[W any](ctx context.Context, c *Client, path string, q url.Values) ([]W, error) {
	panic("not implemented")
}

// Raw is the escape hatch for `bb api <path>`: same session, renewal, write
// gate, and error mapping as every other call; the response body is returned
// unparsed. path must be absolute on the Blackboard host ("/learn/api/...").
// It is the only exported method that exposes wire JSON, and only as bytes.
func (c *Client) Raw(ctx context.Context, method, path string, body io.Reader) (status int, header http.Header, respBody io.ReadCloser, err error) {
	panic("not implemented")
}

// Me returns the logged-in user. Used by `bb auth status` to prove the
// session works and by Courses (memberships are per user id).
func (c *Client) Me(ctx context.Context) (User, error) { panic("not implemented") }

type User struct {
	ID       UserID
	UserName string // NetID
	Name     string // given + family, unescaped
}
