package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/ddvk/rmfakecloud/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

func init() { gin.SetMode(gin.TestMode) }

type stubUserStorer struct{ users map[string]*model.User }

func (s *stubUserStorer) GetUsers() ([]*model.User, error) { return nil, nil }

func (s *stubUserStorer) GetUser(uid string) (*model.User, error) {
	if u, ok := s.users[uid]; ok {
		return u, nil
	}
	return nil, storage.ErrUserNotFound
}

func (s *stubUserStorer) RegisterUser(u *model.User) error { s.users[u.ID] = u; return nil }
func (s *stubUserStorer) UpdateUser(u *model.User) error   { s.users[u.ID] = u; return nil }
func (s *stubUserStorer) RemoveUser(uid string) error      { delete(s.users, uid); return nil }

// deviceTokenFor mints what a paired tablet holds: a token with no expiry that
// it presents every few hours to get a fresh user token.
func deviceTokenFor(t *testing.T, secret []byte, uid string) string {
	t.Helper()
	claims := &DeviceClaims{
		UserID:         uid,
		DeviceDesc:     "remarkable2",
		DeviceID:       "test-device",
		StandardClaims: jwt.StandardClaims{Audience: APIUsage},
	}
	token, err := common.SignClaims(claims, secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func renewUserToken(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	secret := []byte("test-secret")
	app := &App{
		cfg:        &config.Config{JWTSecretKey: secret},
		userStorer: &stubUserStorer{users: map[string]*model.User{user.ID: user}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/token/json/2/user/new", nil)
	c.Request.Header.Set("Authorization", "Bearer "+deviceTokenFor(t, secret, user.ID))

	app.newUserToken(c)
	return rec
}

// The device token never expires, so the three hourly user token renewal is the
// only place the server gets to reconsider. If this does not check the flag,
// disabling an account leaves the tablet syncing forever.
func TestTheTabletStopsRenewingOnceTheAccountIsDisabled(t *testing.T) {
	user := &model.User{ID: "alice", Email: "alice@example.com", Disabled: true}

	rec := renewUserToken(t, user)

	if rec.Code == http.StatusOK {
		t.Fatal("a disabled account was issued a fresh user token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rec.Code)
	}
}

func TestTheTabletKeepsRenewingForAnEnabledAccount(t *testing.T) {
	user := &model.User{ID: "alice", Email: "alice@example.com"}

	rec := renewUserToken(t, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("no token in the response")
	}
}
