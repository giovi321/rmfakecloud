package ui

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func testApp(cfg *config.Config) *ReactAppWrapper {
	if cfg.JWTSecretKey == nil {
		cfg.JWTSecretKey = []byte("test-secret")
	}
	return &ReactAppWrapper{cfg: cfg}
}

func issueFor(t *testing.T, app *ReactAppWrapper, user *model.User) (*gin.Context, *httptest.ResponseRecorder, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/ui/api/login", nil)

	token, err := app.issueWebSession(c, user)
	if err != nil {
		t.Fatalf("issueWebSession: %v", err)
	}
	return c, rec, token
}

func claimsOf(t *testing.T, app *ReactAppWrapper, token string) *WebUserClaims {
	t.Helper()
	claims := &WebUserClaims{}
	if err := common.ClaimsFromToken(claims, token, app.cfg.JWTSecretKey); err != nil {
		t.Fatalf("the issued token does not verify: %v", err)
	}
	return claims
}

func TestWebSessionCarriesTheUserAndTheWebAudience(t *testing.T) {
	app := testApp(&config.Config{})
	user := &model.User{ID: "alice", Email: "alice@example.com"}

	_, _, token := issueFor(t, app, user)
	claims := claimsOf(t, app, token)

	if claims.UserID != "alice" {
		t.Errorf("UserID: got %q", claims.UserID)
	}
	if !slices.Contains(claims.Audience, WebUsage) {
		t.Errorf("audience: got %v, want it to contain %q", claims.Audience, WebUsage)
	}
	if claims.BrowserID == "" {
		t.Error("BrowserID must be set, the sync notifier keys off it")
	}
}

func TestWebSessionGivesTheAdminRoleOnlyToAnAdmin(t *testing.T) {
	app := testApp(&config.Config{})

	_, _, adminToken := issueFor(t, app, &model.User{ID: "root", IsAdmin: true})
	if !slices.Contains(claimsOf(t, app, adminToken).Roles, AdminRole) {
		t.Error("an admin must carry the admin role")
	}

	_, _, userToken := issueFor(t, app, &model.User{ID: "alice"})
	if slices.Contains(claimsOf(t, app, userToken).Roles, AdminRole) {
		t.Error("a plain user must not carry the admin role")
	}
}

func TestWebSessionCarriesTheSync15ScopeOnlyForASync15User(t *testing.T) {
	app := testApp(&config.Config{})

	_, _, token := issueFor(t, app, &model.User{ID: "alice", Sync15: true})
	if !slices.Contains(strings.Fields(claimsOf(t, app, token).Scopes), isSync15Key) {
		t.Error("a sync15 user must carry the sync15 scope")
	}

	_, _, plain := issueFor(t, app, &model.User{ID: "bob"})
	if slices.Contains(strings.Fields(claimsOf(t, app, plain).Scopes), isSync15Key) {
		t.Error("a sync10 user must not carry the sync15 scope")
	}
}

func TestWebSessionCookieIsHTTPOnlyAndFollowsTheSecureSetting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		httpsOnly  bool
		wantSecure bool
	}{
		{"https deployment", true, true},
		{"plain http deployment", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp(&config.Config{HTTPSCookie: tc.httpsOnly})

			_, rec, _ := issueFor(t, app, &model.User{ID: "alice"})

			var found *http.Cookie
			for _, ck := range rec.Result().Cookies() {
				if ck.Name == cookieName {
					found = ck
				}
			}
			if found == nil {
				t.Fatalf("no %s cookie was set", cookieName)
			}
			if !found.HttpOnly {
				t.Error("the session cookie must be HttpOnly, the frontend has no reason to read it")
			}
			if found.Secure != tc.wantSecure {
				t.Errorf("Secure: got %v, want %v", found.Secure, tc.wantSecure)
			}
			if found.Value == "" {
				t.Error("the cookie carries no token")
			}
		})
	}
}
