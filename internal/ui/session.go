package ui

import (
	"net/http"
	"time"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

const webSessionLifetime = 24 * time.Hour

// issueWebSession builds the web session token for user, sets the auth cookie
// and returns the signed token. It is the only place the expiry, the scopes and
// the cookie attributes of a UI session are decided, so password login and OIDC
// login cannot drift apart.
func (app *ReactAppWrapper) issueWebSession(c *gin.Context, user *model.User) (string, error) {
	scopes := ""
	if user.Sync15 {
		scopes = isSync15Key
	}

	roles := []string{"User"}
	if user.IsAdmin {
		roles = []string{AdminRole}
	}

	expires := time.Now().Add(webSessionLifetime)
	claims := &WebUserClaims{
		UserID:    user.ID,
		BrowserID: uuid.NewString(),
		Email:     user.Email,
		Scopes:    scopes,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expires),
			Issuer:    "rmFake WEB",
			Audience:  []string{WebUsage},
		},
	}

	tokenString, err := common.SignClaims(claims, app.cfg.JWTSecretKey)
	if err != nil {
		return "", err
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(cookieName, tokenString, int(webSessionLifetime.Seconds()), "/", "", app.cfg.HTTPSCookie, true)
	return tokenString, nil
}

// clearWebSession removes the auth cookie using the same attributes it was set
// with, so the browser matches it rather than leaving a second one in place.
func (app *ReactAppWrapper) clearWebSession(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(cookieName, "", -1, "/", "", app.cfg.HTTPSCookie, true)
}
