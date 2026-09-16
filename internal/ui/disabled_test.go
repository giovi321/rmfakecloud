package ui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/ddvk/rmfakecloud/internal/storage"
	"github.com/gin-gonic/gin"
)

type fakeUserStorer struct {
	users   map[string]*model.User
	updated []*model.User
}

func newFakeStorer(users ...*model.User) *fakeUserStorer {
	s := &fakeUserStorer{users: map[string]*model.User{}}
	for _, u := range users {
		s.users[u.ID] = u
	}
	return s
}

func (s *fakeUserStorer) GetUsers() ([]*model.User, error) {
	all := make([]*model.User, 0, len(s.users))
	for _, u := range s.users {
		all = append(all, u)
	}
	return all, nil
}

func (s *fakeUserStorer) GetUser(uid string) (*model.User, error) {
	if u, ok := s.users[uid]; ok {
		return u, nil
	}
	return nil, storage.ErrUserNotFound
}

func (s *fakeUserStorer) RegisterUser(u *model.User) error {
	s.users[u.ID] = u
	return nil
}

func (s *fakeUserStorer) UpdateUser(u *model.User) error {
	s.users[u.ID] = u
	s.updated = append(s.updated, u)
	return nil
}

func (s *fakeUserStorer) RemoveUser(uid string) error {
	delete(s.users, uid)
	return nil
}

type fakeCodeGenerator struct{ issued []string }

func (g *fakeCodeGenerator) NewCode(uid string) (string, error) {
	g.issued = append(g.issued, uid)
	return "abcd1234", nil
}

func mustUser(t *testing.T, id, password string) *model.User {
	t.Helper()
	u, err := model.NewUser(id, password)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func appWithStorer(t *testing.T, cfg *config.Config, storer storage.UserStorer) (*ReactAppWrapper, *gin.Engine) {
	t.Helper()
	if cfg.JWTSecretKey == nil {
		cfg.JWTSecretKey = []byte("test-secret")
	}
	app := &ReactAppWrapper{
		cfg:           cfg,
		prefix:        "/assets",
		userStorer:    storer,
		codeConnector: &fakeCodeGenerator{},
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

// The tablet holds a device token that never expires, so revoking access cannot
// mean "wait for the session to lapse". Disabled is the lever, and every door
// into the account has to respect it.
func TestPasswordLoginRefusesADisabledAccount(t *testing.T) {
	user := mustUser(t, "alice", "hunter2")
	user.Disabled = true
	_, router := appWithStorer(t, &config.Config{}, newFakeStorer(user))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/api/login",
		strings.NewReader(`{"email":"alice","password":"hunter2"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("a disabled account logged in with the right password")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", rec.Code)
	}
}

func TestPasswordLoginStillWorksForAnEnabledAccount(t *testing.T) {
	user := mustUser(t, "alice", "hunter2")
	_, router := appWithStorer(t, &config.Config{}, newFakeStorer(user))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/api/login",
		strings.NewReader(`{"email":"alice","password":"hunter2"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestOIDCLoginRefusesADisabledAccount(t *testing.T) {
	user := mustUser(t, "alice", "hunter2")
	user.Disabled = true
	app, _ := appWithStorer(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true}, newFakeStorer(user))

	identity := oidcUserIdentity{Value: "alice", ClaimName: "preferred_username"}
	_, err := app.getOrProvisionUser("alice", identity, oidcClaims{}, nil)

	if !errors.Is(err, errUserDisabled) {
		t.Errorf("got %v, want errUserDisabled; the provider must not be a way around the flag", err)
	}
}

func TestOIDCProvisioningIsUnaffectedByTheFlag(t *testing.T) {
	storer := newFakeStorer()
	app, _ := appWithStorer(t, &config.Config{OIDC: oidcOn(), HTTPSCookie: true}, storer)

	identity := oidcUserIdentity{Value: "newcomer", ClaimName: "preferred_username"}
	user, err := app.getOrProvisionUser("newcomer", identity, oidcClaims{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Disabled {
		t.Error("a newly provisioned account must not start disabled")
	}
}

// A disabled user must not be able to mint an enrolment code, or disabling the
// account would only last until they paired another tablet.
func TestAnEnrolmentCodeIsRefusedForADisabledAccount(t *testing.T) {
	user := mustUser(t, "alice", "hunter2")
	user.Disabled = true
	app, router := appWithStorer(t, &config.Config{}, newFakeStorer(user))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/api/newcode", nil)
	req.AddCookie(sessionCookieFor(t, app, user))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", rec.Code)
	}
}

// The user chose to pay a storage read per request so that revoking access takes
// effect on the web UI immediately rather than after the session expires.
func TestAnExistingSessionStopsWorkingOnceTheAccountIsDisabled(t *testing.T) {
	user := mustUser(t, "alice", "hunter2")
	storer := newFakeStorer(user)
	app, router := appWithStorer(t, &config.Config{}, storer)
	cookie := sessionCookieFor(t, app, user)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/api/me", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: got %d, want the session to work before the flag is set", rec.Code)
	}

	user.Disabled = true

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ui/api/me", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403 on the very next request", rec.Code)
	}
}

func sessionCookieFor(t *testing.T, app *ReactAppWrapper, user *model.User) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/ui/api/login", nil)
	if _, err := app.issueWebSession(c, user); err != nil {
		t.Fatal(err)
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == cookieName {
			return ck
		}
	}
	t.Fatal("no session cookie was issued")
	return nil
}
