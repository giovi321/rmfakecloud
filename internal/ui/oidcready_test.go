package ui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ddvk/rmfakecloud/internal/config"
)

// stubProvider builds a provider without touching the network, which is what
// ProviderConfig is for.
func stubProvider(issuer string) *gooidc.Provider {
	base := strings.TrimSuffix(issuer, "/") + "/"
	return (&gooidc.ProviderConfig{
		IssuerURL: issuer,
		AuthURL:   base + "authorize",
		TokenURL:  base + "token",
		JWKSURL:   base + "jwks",
	}).NewProvider(context.Background())
}

func workingDiscovery(calls *int32) func(context.Context, string) (*gooidc.Provider, error) {
	return func(_ context.Context, issuer string) (*gooidc.Provider, error) {
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		return stubProvider(issuer), nil
	}
}

func failingDiscovery(calls *int32) func(context.Context, string) (*gooidc.Provider, error) {
	return func(context.Context, string) (*gooidc.Provider, error) {
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		return nil, errors.New("dial tcp: connection refused")
	}
}

// An identity provider that is down must not stop the service. rmfakecloud is
// the only thing running on its host, and the tablet's sync does not involve
// the provider at all, so a hard failure here takes out sync for a reason that
// has nothing to do with sync.
func TestAnUnreachableProviderDoesNotStopTheAppServingPages(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	app, router := oidcAppWith(t, cfg, failingDiscovery(nil))
	_ = app

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want the app shell", rec.Code)
	}
}

func TestTheLoginEntryPointReportsAnUnreachableProvider(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	_, router := oidcAppWith(t, cfg, failingDiscovery(nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503 rather than a redirect into a dead flow", rec.Code)
	}
}

func TestTheCallbackReportsAnUnreachableProvider(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	_, router := oidcAppWith(t, cfg, failingDiscovery(nil))

	req := httptest.NewRequest(http.MethodGet, "/ui/api/oidc/callback?code=x&state=y", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "y"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "n"})
	req.AddCookie(&http.Cookie{Name: oidcVerifierCookie, Value: "v"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("got %d, want 503", rec.Code)
	}
}

// The provider coming back must not need a restart of rmfakecloud, or the
// dependency is only moved rather than removed.
func TestTheProviderIsPickedUpOnceItComesBack(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	var down atomic.Bool
	down.Store(true)

	app, router := oidcAppWith(t, cfg, func(_ context.Context, issuer string) (*gooidc.Provider, error) {
		if down.Load() {
			return nil, errors.New("dial tcp: connection refused")
		}
		return stubProvider(issuer), nil
	})
	_ = app

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("setup: got %d, want 503 while the provider is down", rec.Code)
	}

	down.Store(false)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("got %d, want a redirect once the provider is back, with no restart", rec.Code)
	}
}

// Discovery fetches keys over the network, so it must not run on every login.
func TestDiscoveryHappensOnceAndIsThenReused(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	var calls int32
	_, router := oidcAppWith(t, cfg, workingDiscovery(&calls))

	for range 3 {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))
		if rec.Code != http.StatusFound {
			t.Fatalf("got %d, want a redirect", rec.Code)
		}
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("discovery ran %d times, want 1", got)
	}
}

func TestConcurrentLoginsDiscoverOnlyOnce(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	var calls int32
	_, router := oidcAppWith(t, cfg, workingDiscovery(&calls))

	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))
		}()
	}
	for range 8 {
		<-done
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("discovery ran %d times under concurrent logins, want 1", got)
	}
}
