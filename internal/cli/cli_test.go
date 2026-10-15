package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/controlled-opposition/billboard/internal/auth"
	"github.com/controlled-opposition/billboard/internal/cli"
)

// Synthetic Blackboard: shapes follow the captured public REST responses;
// every name and id is invented.
const (
	meJSON = `{"id":"_901_1","userName":"jdoe","name":{"given":"Jo","family":"Doe"}}`

	membershipsPage1 = `{"results":[
		{"courseId":"_501_1","courseRoleId":"Student","availability":{"available":"Yes"},
		 "course":{"id":"_501_1","courseId":"SOC101-01-F26","name":"Data & Society","termId":"_10_1",
		           "externalAccessUrl":"https://bb.example/ultra/courses/_501_1/outline","availability":{"available":"Yes"}}},
		{"courseId":"_502_1","courseRoleId":"Student","availability":{"available":"Yes"},
		 "course":{"id":"_502_1","courseId":"MATH211-01-F26","name":"Linear Algebra","termId":"_10_1",
		           "externalAccessUrl":"https://bb.example/ultra/courses/_502_1/outline","availability":{"available":"Yes"}}}
	],"paging":{"nextPage":"/learn/api/public/v1/users/_901_1/courses?expand=course&offset=2"}}`

	membershipsPage2 = `{"results":[
		{"courseId":"_503_1","courseRoleId":"Student","availability":{"available":"Yes"},
		 "course":{"id":"_503_1","courseId":"MATH211-02-F26","name":"Linear Algebra","termId":"_10_1",
		           "externalAccessUrl":"https://bb.example/ultra/courses/_503_1/outline","availability":{"available":"Yes"}}},
		{"courseId":"_404_1","courseRoleId":"Student","availability":{"available":"Disabled"},
		 "course":{"id":"_404_1","courseId":"HIST150-01-S25","name":"World History","termId":"_11_1",
		           "externalAccessUrl":"https://bb.example/ultra/courses/_404_1/outline","availability":{"available":"Yes"}}}
	]}`

	termsJSON = `{"results":[{"id":"_10_1","name":"Fall 2026"},{"id":"_11_1","name":"Spring 2025"}]}`

	forbiddenJSON = `{"status":403,"message":"You do not have permission to access grade column"}`
)

func bbRouter(xsrf string) string {
	return "expires%3A1800000000%2Cid%3Asyn123%2CsessionId%3A42%2Cxsrf%3A" + xsrf
}

type memKeyring struct {
	mu sync.Mutex
	b  []byte
}

func (k *memKeyring) Get() ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.b == nil {
		return nil, auth.ErrNoItem
	}
	return k.b, nil
}
func (k *memKeyring) Set(b []byte) error { k.mu.Lock(); k.b = b; k.mu.Unlock(); return nil }
func (k *memKeyring) Delete() error      { k.mu.Lock(); k.b = nil; k.mu.Unlock(); return nil }

// harness is one simulated machine: a Blackboard server that accepts one
// xsrf token, a keychain, a config dir, and a fake Chrome that hands out
// the next session from xsrfs on every launch.
type harness struct {
	t        *testing.T
	srv      *httptest.Server
	dir      string
	keyring  *memKeyring
	valid    string            // the xsrf the server accepts
	xsrfs    []string          // sessions the fake Chrome yields, in order
	launches []string          // "visible" or "headless", per Chrome launch
	requests []string          // request URIs the server saw
	cookies  []string          // Cookie headers the server saw
	tty      bool              // stdout is a terminal of unknown width
	routes   map[string]string // extra responses by request URI
	fail     map[string]int    // error statuses by request URI
}

func newHarness(t *testing.T, valid string, xsrfs ...string) *harness {
	t.Parallel()
	h := &harness{t: t, dir: t.TempDir(), keyring: &memKeyring{}, valid: valid, xsrfs: xsrfs}
	h.srv = httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *harness) serve(w http.ResponseWriter, r *http.Request) {
	h.requests = append(h.requests, r.URL.RequestURI())
	h.cookies = append(h.cookies, r.Header.Get("Cookie"))
	if r.Header.Get("X-Blackboard-XSRF") != h.valid {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status":401,"message":"API request is not authorized."}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if status, ok := h.fail[r.URL.RequestURI()]; ok {
		w.WriteHeader(status)
		if status == http.StatusForbidden {
			fmt.Fprint(w, forbiddenJSON)
		} else {
			fmt.Fprintf(w, `{"status":%d,"message":%q}`, status, http.StatusText(status))
		}
		return
	}
	switch r.URL.RequestURI() {
	case "/learn/api/public/v1/users/me":
		fmt.Fprint(w, meJSON)
	case "/learn/api/public/v1/users/_901_1/courses?expand=course":
		fmt.Fprint(w, membershipsPage1)
	case "/learn/api/public/v1/users/_901_1/courses?expand=course&offset=2":
		fmt.Fprint(w, membershipsPage2)
	case "/learn/api/public/v1/terms":
		fmt.Fprint(w, termsJSON)
	default:
		if body, ok := h.routes[r.URL.RequestURI()]; ok {
			fmt.Fprint(w, body)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":404,"message":"Not found"}`)
	}
}

func (h *harness) browser(_ context.Context, r auth.BrowserRun) ([]*http.Cookie, error) {
	mode := "visible"
	if r.Headless {
		mode = "headless"
	}
	h.launches = append(h.launches, mode)
	if len(h.xsrfs) == 0 {
		return nil, fmt.Errorf("sign-in page asks for credentials or MFA: %w", auth.ErrNeedsHuman)
	}
	x := h.xsrfs[0]
	h.xsrfs = h.xsrfs[1:]
	return []*http.Cookie{{Name: "AWSALB", Value: "syn-lb"}, {Name: "BbRouter", Value: bbRouter(x)}}, nil
}

func (h *harness) run(env map[string]string, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = cli.Execute(context.Background(), cli.Env{
		Stdout:    &out,
		Stderr:    &errb,
		StdoutTTY: h.tty,
		Getenv:    func(k string) string { return env[k] },
		Now:       func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) },
		BaseURL:   h.srv.URL,
		ConfigDir: h.dir,
		Keyring:   h.keyring,
		Browser:   h.browser,
	}, args)
	return code, out.String(), errb.String()
}

func (h *harness) login() {
	h.t.Helper()
	code, _, stderr := h.run(nil, "auth", "login")
	want := "bb: complete the NetID + MFA login in the Chrome window (timeout 5m0s)\nbb: logged in\n"
	if code != 0 || stderr != want {
		h.t.Fatalf("auth login: code=%d stderr=%q, want 0 and %q", code, stderr, want)
	}
}

func expect(t *testing.T, gotCode int, gotOut, gotErr string, wantCode int, wantOut, wantErr string) {
	t.Helper()
	if gotCode != wantCode {
		t.Errorf("exit code = %d, want %d", gotCode, wantCode)
	}
	if gotOut != wantOut {
		t.Errorf("stdout:\n got %q\nwant %q", gotOut, wantOut)
	}
	if gotErr != wantErr {
		t.Errorf("stderr:\n got %q\nwant %q", gotErr, wantErr)
	}
}

func TestCourseListTSVPages(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	code, out, errOut := h.run(nil, "course", "list")
	expect(t, code, out, errOut, 0,
		"MATH211-01-F26\tLinear Algebra\tFall 2026\n"+
			"MATH211-02-F26\tLinear Algebra\tFall 2026\n"+
			"SOC101-01-F26\tData & Society\tFall 2026\n", "")
	wantReqs := []string{
		"/learn/api/public/v1/users/me",
		"/learn/api/public/v1/users/_901_1/courses?expand=course",
		"/learn/api/public/v1/users/_901_1/courses?expand=course&offset=2",
		"/learn/api/public/v1/terms",
	}
	if got := strings.Join(h.requests, "\n"); got != strings.Join(wantReqs, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", got, strings.Join(wantReqs, "\n"))
	}
}

func TestCourseListTableOnTerminal(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	h.tty = true
	code, out, errOut := h.run(nil, "course", "list")
	expect(t, code, out, errOut, 0,
		"CODE            NAME            TERM\n"+
			"MATH211-01-F26  Linear Algebra  Fall 2026\n"+
			"MATH211-02-F26  Linear Algebra  Fall 2026\n"+
			"SOC101-01-F26   Data & Society  Fall 2026\n", "")
}

func TestCourseListAllJSON(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	code, out, errOut := h.run(nil, "course", "list", "--all", "--json", "code,name,current")
	expect(t, code, out, errOut, 0,
		`[{"code":"MATH211-01-F26","current":true,"name":"Linear Algebra"},`+
			`{"code":"MATH211-02-F26","current":true,"name":"Linear Algebra"},`+
			`{"code":"SOC101-01-F26","current":true,"name":"Data & Society"},`+
			`{"code":"HIST150-01-S25","current":false,"name":"World History"}]`+"\n", "")
}

func TestCourseListJQ(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	code, out, errOut := h.run(nil, "course", "list", "--json", "code,name", "--jq", `.[] | select(.name | test("&")) | .code`)
	expect(t, code, out, errOut, 0, "SOC101-01-F26\n", "")
}

func TestAmbiguousCourseListsCandidates(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	code, out, errOut := h.run(nil, "course", "list", "math211")
	expect(t, code, out, errOut, 5, "",
		"bb: course \"math211\" matches 2 courses; use a longer code or the id\n"+
			"  MATH211-01-F26\t_502_1\tLinear Algebra\tFall 2026\n"+
			"  MATH211-02-F26\t_503_1\tLinear Algebra\tFall 2026\n")

	code, out, errOut = h.run(nil, "course", "list", "math211-02")
	expect(t, code, out, errOut, 0, "MATH211-02-F26\tLinear Algebra\tFall 2026\n", "")
}

func TestExpiredSessionRenewsHeadlesslyAndRetries(t *testing.T) {
	h := newHarness(t, "x2", "x1", "x2")
	h.login()
	code, out, errOut := h.run(nil, "auth", "status")
	expect(t, code, out, errOut, 0, "true\tjdoe\t2027-01-15T08:00:00Z\n", "")
	if got := strings.Join(h.launches, ","); got != "visible,headless" {
		t.Errorf("chrome launches = %s, want visible,headless", got)
	}
	if got := strings.Join(h.requests, ","); got != "/learn/api/public/v1/users/me,/learn/api/public/v1/users/me" {
		t.Errorf("requests = %s, want users/me rejected then retried", got)
	}

	// The renewed session was saved: the next process uses it without Chrome.
	code, out, errOut = h.run(nil, "api", "/learn/api/public/v1/users/me", "--jq", ".userName")
	expect(t, code, out, errOut, 0, "jdoe\n", "")
	if len(h.launches) != 2 {
		t.Errorf("chrome launched %d times, want 2", len(h.launches))
	}
}

func TestExpiredSessionWithNoRenewNeedsHuman(t *testing.T) {
	h := newHarness(t, "x2", "x1", "x2")
	h.login()
	code, out, errOut := h.run(map[string]string{"BB_NO_RENEW": "1"}, "course", "list")
	expect(t, code, out, errOut, 4, "",
		"bb: session expired and BB_NO_RENEW is set: blackboard login required: a human must run `bb auth login`\n"+
			"bb: ask the human to run `bb auth login`, then retry\n")
	if got := strings.Join(h.launches, ","); got != "visible" {
		t.Errorf("chrome launches = %s, want only the login", got)
	}
}

func TestRenewalThatNeedsMFAExitsFour(t *testing.T) {
	h := newHarness(t, "x2", "x1")
	h.login()
	code, out, errOut := h.run(nil, "course", "list")
	expect(t, code, out, errOut, 4, "",
		"bb: headless renewal failed: sign-in page asks for credentials or MFA: blackboard login required: a human must run `bb auth login`\n"+
			"bb: ask the human to run `bb auth login`, then retry\n")
}

func TestForbiddenHasItsOwnExitCode(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.fail = map[string]int{"/learn/api/public/v2/courses/_501_1/gradebook/columns": http.StatusForbidden}
	h.login()
	code, out, errOut := h.run(nil, "api", "/learn/api/public/v2/courses/_501_1/gradebook/columns")
	expect(t, code, out, errOut, 3, "", "bb: HTTP 403: You do not have permission to access grade column\n")

	code, out, errOut = h.run(nil, "api", "/learn/api/public/v1/courses/_999_1")
	expect(t, code, out, errOut, 5, "", "bb: HTTP 404: Not found\n")
}

func TestAPISendsCookiesAndXSRFFromBbRouter(t *testing.T) {
	h := newHarness(t, "0f1e2d3c-1111-2222-3333-444455556666", "0f1e2d3c-1111-2222-3333-444455556666")
	h.login()
	code, out, errOut := h.run(nil, "api", "learn/api/public/v1/users/me")
	expect(t, code, out, errOut, 0, meJSON, "")
	want := "AWSALB=syn-lb; BbRouter=" + bbRouter("0f1e2d3c-1111-2222-3333-444455556666")
	if got := h.cookies[len(h.cookies)-1]; got != want {
		t.Errorf("Cookie header = %q, want %q", got, want)
	}
}

func TestAPIRefusesOtherHosts(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	for _, arg := range []string{"//evil.example/steal", "https://example.com/x", "http://evil.example/steal", "mailto:jdoe@example.com"} {
		code, out, errOut := h.run(nil, "api", arg)
		expect(t, code, out, errOut, 2, "", fmt.Sprintf("bb: api takes a path on the Blackboard host, not a URL: %q\n", arg))
	}
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want no requests", h.requests)
	}
}

func TestMaxRequestsStopsTheWalk(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	code, out, errOut := h.run(nil, "--max-requests", "2", "course", "list")
	expect(t, code, out, errOut, 1, "",
		"bb: request cap reached; raise it with --max-requests if this walk really needs more\n")
	if len(h.requests) != 2 {
		t.Errorf("server saw %d requests, want 2", len(h.requests))
	}
}

func TestStatusWithoutSessionAndIdempotentLogout(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	h.login()
	for range 2 {
		code, out, errOut := h.run(nil, "auth", "logout")
		expect(t, code, out, errOut, 0, "", "bb: logged out\n")
	}
	if _, err := os.Stat(filepath.Join(h.dir, "chrome-profile")); !os.IsNotExist(err) {
		t.Errorf("chrome profile still present: %v", err)
	}
	code, out, errOut := h.run(nil, "auth", "status", "--json", "loggedIn,canRenew")
	expect(t, code, out, errOut, 4, `[{"canRenew":false,"loggedIn":false}]`+"\n",
		"bb: not logged in: blackboard login required: a human must run `bb auth login`\n"+
			"bb: ask the human to run `bb auth login`, then retry\n")
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want no requests without a session", h.requests)
	}
}

func TestUsageErrorsSendNothing(t *testing.T) {
	h := newHarness(t, "x1", "x1")
	cases := []struct {
		args []string
		err  string
	}{
		{[]string{"course", "list", "--json", "title"}, "bb: unknown JSON field \"title\"; available: id, code, name, term, role, current, url\n"},
		{[]string{"course", "list", "--jq", ".[]"}, "bb: --jq and --template require --json\n"},
		{[]string{"course", "lst"}, "bb: unknown command \"lst\" for \"bb course\"\n"},
		{[]string{"api"}, "bb: accepts 1 arg(s), received 0\n"},
	}
	for _, c := range cases {
		code, out, errOut := h.run(nil, c.args...)
		expect(t, code, out, errOut, 2, "", c.err)
	}
	if len(h.requests) != 0 {
		t.Errorf("server saw %v, want none", h.requests)
	}
}
