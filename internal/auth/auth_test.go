package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

type memKeyring struct {
	mu sync.Mutex
	b  []byte
}

func (k *memKeyring) Get() ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.b == nil {
		return nil, ErrNoItem
	}
	return k.b, nil
}

func (k *memKeyring) Set(b []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.b = b
	return nil
}

func (k *memKeyring) Delete() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.b = nil
	return nil
}

func router(v string) []*http.Cookie {
	return []*http.Cookie{{Name: "AWSALB", Value: "lb"}, {Name: "BbRouter", Value: v}}
}

func TestParseSessionReadsXSRFAndExpiry(t *testing.T) {
	cases := []struct {
		name, router, xsrf string
		expires            time.Time
	}{
		{"plain", "expires:1792020617,id:ABC,sessionId:123,xsrf:0f1e2d3c-aaaa-bbbb-cccc-000000000001",
			"0f1e2d3c-aaaa-bbbb-cccc-000000000001", time.Unix(1792020617, 0).UTC()},
		{"url-encoded", "expires%3A1792020617%2Cid%3AABC%2Cxsrf%3Aabc-123",
			"abc-123", time.Unix(1792020617, 0).UTC()},
		{"no expiry", "id:ABC,xsrf:only", "only", time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := parseSession(router(c.router), time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			s.Apply(r)
			if got := r.Header.Get("X-Blackboard-XSRF"); got != c.xsrf {
				t.Errorf("X-Blackboard-XSRF = %q, want %q", got, c.xsrf)
			}
			if got, want := r.Header.Get("Cookie"), "AWSALB=lb; BbRouter="+c.router; got != want {
				t.Errorf("Cookie = %q, want %q", got, want)
			}
			if !s.ExpiresAt().Equal(c.expires) {
				t.Errorf("ExpiresAt = %v, want %v", s.ExpiresAt(), c.expires)
			}
		})
	}
}

func TestParseSessionRejectsMissingXSRF(t *testing.T) {
	if _, err := parseSession(router("expires:1,id:ABC"), time.Time{}); err == nil || err.Error() != "BbRouter cookie has no xsrf field" {
		t.Fatalf("err = %v", err)
	}
	if _, err := parseSession([]*http.Cookie{{Name: "AWSALB", Value: "lb"}}, time.Time{}); err == nil || err.Error() != "no BbRouter cookie" {
		t.Fatalf("err = %v", err)
	}
}

func TestRenewReturnsSessionAnotherProcessSaved(t *testing.T) {
	dir := t.TempDir()
	kr := &memKeyring{}
	launches := 0
	m := NewManager(Options{Paths: PathsIn(dir), Keyring: kr, Browser: func(ctx context.Context, r BrowserRun) ([]*http.Cookie, error) {
		launches++
		return router("xsrf:first"), nil
	}})
	stale, err := m.Login(context.Background(), LoginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	newer, _ := parseSession(router("xsrf:second"), time.Time{})
	b, _ := newer.marshal()
	kr.Set(b)

	got, err := m.Renew(context.Background(), stale)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	got.Apply(r)
	if x := r.Header.Get("X-Blackboard-XSRF"); x != "second" || launches != 1 {
		t.Fatalf("xsrf=%q launches=%d, want second and 1 (login only)", x, launches)
	}
}

func TestRenewWithoutProfileNeedsHuman(t *testing.T) {
	m := NewManager(Options{Paths: PathsIn(t.TempDir()), Keyring: &memKeyring{}, Browser: func(context.Context, BrowserRun) ([]*http.Cookie, error) {
		t.Fatal("browser launched without a profile")
		return nil, nil
	}})
	stale, _ := parseSession(router("xsrf:old"), time.Time{})
	_, err := m.Renew(context.Background(), stale)
	if !errors.Is(err, ErrNeedsHuman) {
		t.Fatalf("err = %v, want ErrNeedsHuman", err)
	}
}

func TestRenewDeadlineCoversBrowserSoWaiterRenews(t *testing.T) {
	dir := t.TempDir()
	kr := &memKeyring{}
	a := NewManager(Options{Paths: PathsIn(dir), Keyring: kr, Browser: func(context.Context, BrowserRun) ([]*http.Cookie, error) {
		return router("xsrf:first"), nil
	}})
	stale, err := a.Login(context.Background(), LoginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a.browser = func(ctx context.Context, r BrowserRun) ([]*http.Cookie, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return router("xsrf:late"), nil
		}
	}
	b := NewManager(Options{Paths: PathsIn(dir), Keyring: kr, Browser: func(context.Context, BrowserRun) ([]*http.Cookie, error) {
		return router("xsrf:from-b"), nil
	}})
	a.renewTimeout = 200 * time.Millisecond
	b.renewTimeout = 200 * time.Millisecond

	start := time.Now()
	aDone := make(chan time.Duration, 1)
	go func() {
		a.Renew(context.Background(), stale)
		aDone <- time.Since(start)
	}()
	time.Sleep(50 * time.Millisecond)
	got, errB := b.Renew(context.Background(), stale)
	held := <-aDone

	if held > 2*time.Second {
		t.Errorf("wedged renewal held the lock for %s, want about 200ms", held.Round(time.Millisecond))
	}
	if errB != nil {
		t.Fatalf("waiting process: %v, want the session it renewed", errB)
	}
	r := httptest.NewRequest("GET", "/", nil)
	got.Apply(r)
	if x := r.Header.Get("X-Blackboard-XSRF"); x != "from-b" {
		t.Fatalf("xsrf = %q, want from-b", x)
	}
}

func TestRenewInterruptedIsNotNeedsHuman(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(PathsIn(dir).Profile, 0o700); err != nil {
		t.Fatal(err)
	}
	m := NewManager(Options{Paths: PathsIn(dir), Keyring: &memKeyring{}, Browser: func(ctx context.Context, r BrowserRun) ([]*http.Cookie, error) {
		<-ctx.Done()
		return nil, errors.New("open login page: chrome exited")
	}})
	stale, _ := parseSession(router("xsrf:old"), time.Time{})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := m.Renew(ctx, stale)
	if errors.Is(err, ErrNeedsHuman) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled and not ErrNeedsHuman", err)
	}
}
