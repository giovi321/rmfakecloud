package ui

import (
	"encoding/json"
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

func adminAppWithStorer(t *testing.T, users ...*model.User) (*ReactAppWrapper, *gin.Engine, *fakeUserStorer) {
	t.Helper()
	admin := mustUser(t, "root", "hunter2")
	admin.IsAdmin = true
	storer := newFakeStorer(append(users, admin)...)
	app, router := appWithStorer(t, &config.Config{}, storer)
	return app, router, storer
}

func adminRequest(t *testing.T, app *ReactAppWrapper, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	admin, err := app.userStorer.GetUser("root")
	if err != nil {
		t.Fatal(err)
	}
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookieFor(t, app, admin))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// Without this the flag can only be set by editing the profile file by hand,
// which is not a revocation procedure anybody will follow under pressure.
func TestAnAdminCanDisableAndReEnableAnAccount(t *testing.T) {
	alice := mustUser(t, "alice", "hunter2")
	app, router, storer := adminAppWithStorer(t, alice)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"alice","disabled":true}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("disable: got %d, want 202", rec.Code)
	}
	if !storer.users["alice"].Disabled {
		t.Fatal("the flag was not stored")
	}

	rec = adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"alice","disabled":false}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("re-enable: got %d, want 202", rec.Code)
	}
	if storer.users["alice"].Disabled {
		t.Error("the account was not re-enabled; revocation has to be reversible")
	}
}

func TestTheUserListShowsWhoIsDisabled(t *testing.T) {
	alice := mustUser(t, "alice", "hunter2")
	alice.Disabled = true
	app, router, _ := adminAppWithStorer(t, alice)

	rec := adminRequest(t, app, router, http.MethodGet, "/ui/api/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}

	var list []struct {
		ID       string `json:"userid"`
		Disabled bool   `json:"disabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}

	found := false
	for _, u := range list {
		if u.ID == "alice" {
			found = true
			if !u.Disabled {
				t.Error("alice is disabled but the list does not say so")
			}
		}
	}
	if !found {
		t.Fatalf("alice missing from %q", rec.Body.String())
	}
}

// An edit that says nothing about the flag must not change it. With a plain
// bool, a password change or an email edit would quietly re-enable a revoked
// account, and nothing in the request would show that it had.
func TestAnUnrelatedEditLeavesTheDisabledFlagAlone(t *testing.T) {
	alice := mustUser(t, "alice", "hunter2")
	alice.Disabled = true
	app, router, storer := adminAppWithStorer(t, alice)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"alice","email":"alice@example.com"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	if !storer.users["alice"].Disabled {
		t.Error("an unrelated edit re-enabled a revoked account")
	}
}

// Disabling the last enabled admin leaves nobody who can undo it: the web UI is
// the only place the flag can be cleared, and every admin session is refused the
// moment the flag is set. Recovery then means editing a profile file on disk.
func TestTheLastEnabledAdminCannotBeDisabled(t *testing.T) {
	app, router, storer := adminAppWithStorer(t)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"root","disabled":true}`)

	if rec.Code == http.StatusAccepted {
		t.Fatal("the only admin disabled themselves, locking the instance")
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("got %d, want 409", rec.Code)
	}
	if storer.users["root"].Disabled {
		t.Error("the flag was stored anyway")
	}
}

func TestAnAdminCanBeDisabledWhileAnotherEnabledAdminRemains(t *testing.T) {
	second := mustUser(t, "second", "hunter2")
	second.IsAdmin = true
	app, router, storer := adminAppWithStorer(t, second)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"second","disabled":true}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	if !storer.users["second"].Disabled {
		t.Error("the flag was not stored")
	}
}

func TestAnAlreadyDisabledAdminDoesNotCountAsCover(t *testing.T) {
	spare := mustUser(t, "spare", "hunter2")
	spare.IsAdmin = true
	spare.Disabled = true
	app, router, _ := adminAppWithStorer(t, spare)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"root","disabled":true}`)

	if rec.Code != http.StatusConflict {
		t.Errorf("got %d, want 409; a disabled admin cannot undo anything", rec.Code)
	}
}

func TestDisablingAPlainUserIsNeverBlocked(t *testing.T) {
	alice := mustUser(t, "alice", "hunter2")
	app, router, storer := adminAppWithStorer(t, alice)

	rec := adminRequest(t, app, router, http.MethodPut, "/ui/api/users",
		`{"userid":"alice","disabled":true}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202", rec.Code)
	}
	if !storer.users["alice"].Disabled {
		t.Error("the flag was not stored")
	}
}
