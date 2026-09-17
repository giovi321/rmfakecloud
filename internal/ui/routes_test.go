package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/gin-gonic/gin"
)

func routerFor(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	_, router := oidcAppWith(t, cfg, workingDiscovery(nil))
	return router
}

// oidcAppWith builds a wrapper whose provider discovery is supplied by the
// caller, so the routing tests never reach the network and can describe a
// provider that is down.
func oidcAppWith(t *testing.T, cfg *config.Config, discover func(context.Context, string) (*gooidc.Provider, error)) (*ReactAppWrapper, *gin.Engine) {
	t.Helper()
	if cfg.JWTSecretKey == nil {
		cfg.JWTSecretKey = []byte("test-secret")
	}
	app := &ReactAppWrapper{
		cfg:          cfg,
		prefix:       "/assets",
		discoverOIDC: discover,
		fs: http.FS(fstest.MapFS{
			"index.html":  {Data: []byte("<html>app</html>")},
			"favicon.ico": {Data: []byte("icon")},
			"robots.txt":  {Data: []byte("")},
		}),
	}

	router := gin.New()
	app.RegisterRoutes(router)
	return app, router
}

func status(t *testing.T, router *gin.Engine, method, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Code
}

func oidcOn() config.OIDCConfig {
	return config.OIDCConfig{
		ProviderURL:  "https://sso.example.com",
		ClientID:     "rmfakecloud",
		ClientSecret: "secret",
		RedirectURL:  "https://rm.example.com" + config.OIDCCallbackPath,
		UserIDClaim:  config.DefaultOIDCUserIDClaim,
	}
}

func TestWithoutOIDCTheLoginRoutesExistAndTheOIDCOnesDoNot(t *testing.T) {
	router := routerFor(t, &config.Config{})

	if got := status(t, router, http.MethodPost, "/ui/api/login"); got == http.StatusNotFound {
		t.Error("password login must be registered when OIDC is off")
	}
	if got := status(t, router, http.MethodGet, "/ui/api/oidc/login"); got != http.StatusNotFound {
		t.Errorf("the OIDC entry point must not exist when OIDC is off, got %d", got)
	}
}

func TestOIDCAloneLeavesPasswordLoginRegistered(t *testing.T) {
	router := routerFor(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true})

	if got := status(t, router, http.MethodPost, "/ui/api/login"); got == http.StatusNotFound {
		t.Error("enabling OIDC must not remove password login on its own")
	}
	if got := status(t, router, http.MethodGet, "/ui/api/oidc/login"); got == http.StatusNotFound {
		t.Error("the OIDC entry point must be registered")
	}
}

func TestDisablingLocalLoginRemovesThePasswordAndRegisterRoutes(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	cfg.OIDC.DisableLocalLogin = true
	router := routerFor(t, cfg)

	if got := status(t, router, http.MethodPost, "/ui/api/login"); got != http.StatusNotFound {
		t.Errorf("password login must be gone, got %d", got)
	}
	if got := status(t, router, http.MethodPost, "/ui/api/register"); got != http.StatusNotFound {
		t.Errorf("registration must be gone, got %d", got)
	}
	if got := status(t, router, http.MethodGet, "/ui/api/oidc/login"); got == http.StatusNotFound {
		t.Error("the OIDC entry point must still be registered")
	}
}

func TestTheOIDCEntryPointRedirectsToTheProvider(t *testing.T) {
	router := routerFor(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("got %d, want a redirect", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc == "" ||
		loc[:len("https://sso.example.com/authorize")] != "https://sso.example.com/authorize" {
		t.Errorf("Location: got %q", loc)
	}
}

func TestOIDCInfoIsAlwaysServedSoTheLoginPageKnowsWhatToRender(t *testing.T) {
	type info struct {
		Enabled           bool   `json:"enabled"`
		DisplayName       string `json:"displayName"`
		LocalLoginEnabled bool   `json:"localLoginEnabled"`
	}

	read := func(t *testing.T, cfg *config.Config) info {
		t.Helper()
		rec := httptest.NewRecorder()
		routerFor(t, cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ui/api/oidc/info", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; the login page has no other way to find out", rec.Code)
		}
		var got info
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
		return got
	}

	off := read(t, &config.Config{})
	if off.Enabled || !off.LocalLoginEnabled {
		t.Errorf("OIDC off: got %+v", off)
	}

	on := read(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true})
	if !on.Enabled || !on.LocalLoginEnabled {
		t.Errorf("OIDC on: got %+v", on)
	}
	if on.DisplayName != config.DefaultOIDCDisplayName {
		t.Errorf("DisplayName: got %q", on.DisplayName)
	}

	exclusiveCfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	exclusiveCfg.OIDC.DisableLocalLogin = true
	exclusiveCfg.OIDC.DisplayName = "Login with Authelia"
	exclusive := read(t, exclusiveCfg)
	if !exclusive.Enabled || exclusive.LocalLoginEnabled {
		t.Errorf("OIDC exclusive: got %+v", exclusive)
	}
	if exclusive.DisplayName != "Login with Authelia" {
		t.Errorf("DisplayName: got %q", exclusive.DisplayName)
	}
}

func TestAnUnauthenticatedPageIsServedNormallyWhilePasswordLoginExists(t *testing.T) {
	router := routerFor(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want the app shell so the user can choose how to log in", rec.Code)
	}
}

func TestWithLocalLoginOffAnUnauthenticatedPageGoesStraightToTheProvider(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	cfg.OIDC.DisableLocalLogin = true
	router := routerFor(t, cfg)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/documents", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("got %d, want a redirect to the provider", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/ui/api/oidc/login" {
		t.Errorf("Location: got %q", got)
	}
}

// Without this exclusion the logout landing page redirects back to the provider,
// which signs the user straight back in and makes logging out impossible.
func TestTheLoggedOutPageIsNotRedirectedBackToTheProvider(t *testing.T) {
	cfg := &config.Config{OIDC: oidcOn(), HTTPSCookie: true}
	cfg.OIDC.DisableLocalLogin = true
	router := routerFor(t, cfg)

	for _, path := range []string{"/logged-out", "/oidc-success"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want the app shell", path, rec.Code)
		}
	}
}
