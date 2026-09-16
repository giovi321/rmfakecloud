package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/ddvk/rmfakecloud/internal/storage"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// errUserDisabled is returned when the account exists but has been revoked.
var errUserDisabled = errors.New("account disabled")

const (
	oidcLog = "[oidc] "

	oidcStateCookie    = "oidc_state"
	oidcNonceCookie    = "oidc_nonce"
	oidcVerifierCookie = "oidc_pkce_verifier"
	// oidcFlowCookieSeconds bounds how long a half-finished login stays valid.
	oidcFlowCookieSeconds = 300

	// OIDCSuccessPath is where the callback sends the browser once a session
	// exists. The frontend has a route of the same name.
	OIDCSuccessPath = "/oidc-success"
	// OIDCLoggedOutPath is the landing page after logging out. It exists so an
	// instance with local login disabled does not redirect a user who has just
	// logged out straight back to the provider, which would sign them in again.
	OIDCLoggedOutPath = "/logged-out"
	// OIDCLoginPath is the entry point that starts the flow.
	OIDCLoginPath = "/ui/api/oidc/login"
)

// randomURLSafeString returns n cryptographically random bytes, base64url encoded.
func randomURLSafeString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// setOIDCCookie writes a short lived flow cookie. SameSite has to be Lax rather
// than Strict: the browser arrives back from the provider on a cross site
// top level redirect, and a Strict cookie is not sent on one.
func (app *ReactAppWrapper) setOIDCCookie(c *gin.Context, name, value string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, oidcFlowCookieSeconds, "/", "", app.cfg.HTTPSCookie, true)
}

func (app *ReactAppWrapper) clearOIDCCookie(c *gin.Context, name string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/", "", app.cfg.HTTPSCookie, true)
}

// oidcInfo tells the login page what to render. The frontend is a static bundle
// compiled into the binary, so it cannot read the environment itself.
func (app *ReactAppWrapper) oidcInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":           app.cfg.OIDC.Enabled(),
		"displayName":       app.cfg.OIDC.Label(),
		"localLoginEnabled": app.cfg.OIDC.LocalLoginEnabled(),
	})
}

// oidcBegin starts the authorization code flow with PKCE, state and nonce.
func (app *ReactAppWrapper) oidcBegin(c *gin.Context) {
	state, err := randomURLSafeString(32)
	if err != nil {
		log.Error(oidcLog, "generating state: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	nonce, err := randomURLSafeString(32)
	if err != nil {
		log.Error(oidcLog, "generating nonce: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	verifier, err := randomURLSafeString(32)
	if err != nil {
		log.Error(oidcLog, "generating pkce verifier: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	app.setOIDCCookie(c, oidcStateCookie, state)
	app.setOIDCCookie(c, oidcNonceCookie, nonce)
	app.setOIDCCookie(c, oidcVerifierCookie, verifier)

	c.Redirect(http.StatusFound, app.oauth2Config.AuthCodeURL(
		state,
		gooidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	))
}

// oidcCallback handles the redirect back from the provider. It settles the
// protocol level checks, then hands identity and provisioning to completeOIDCLogin.
func (app *ReactAppWrapper) oidcCallback(c *gin.Context) {
	ctx := c.Request.Context()

	// The provider's own error text is not repeated back to the browser.
	if errParam := c.Query("error"); errParam != "" {
		log.Warn(oidcLog, "provider error: ", errParam, " ", c.Query("error_description"))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication failed at the identity provider"})
		return
	}

	stateCookie, err := c.Cookie(oidcStateCookie)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing state cookie"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(stateCookie), []byte(c.Query("state"))) != 1 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "authentication failed"})
		return
	}

	nonceCookie, err := c.Cookie(oidcNonceCookie)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing nonce cookie"})
		return
	}
	verifier, err := c.Cookie(oidcVerifierCookie)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing pkce verifier cookie"})
		return
	}

	app.clearOIDCCookie(c, oidcStateCookie)
	app.clearOIDCCookie(c, oidcNonceCookie)
	app.clearOIDCCookie(c, oidcVerifierCookie)

	rawClaims, claims, ok := app.exchangeAndVerifyToken(c, ctx, c.Query("code"), verifier)
	if !ok {
		return
	}

	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonceCookie)) != 1 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "authentication failed"})
		return
	}

	app.completeOIDCLogin(c, rawClaims, claims)
}

// exchangeAndVerifyToken trades the authorization code for tokens and verifies
// the ID token. It returns the raw claim map, for configurable dotted paths, and
// the standard claims. On failure the response has already been written.
func (app *ReactAppWrapper) exchangeAndVerifyToken(c *gin.Context, ctx context.Context, code, verifier string) (map[string]any, oidcClaims, bool) {
	token, err := app.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Error(oidcLog, "token exchange failed: ", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token exchange failed"})
		return nil, oidcClaims{}, false
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no id_token in the provider response"})
		return nil, oidcClaims{}, false
	}

	idToken, err := app.oidcProvider.Verifier(&gooidc.Config{ClientID: app.cfg.OIDC.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		log.Warn(oidcLog, "id token verification failed: ", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "id token verification failed"})
		return nil, oidcClaims{}, false
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		log.Error(oidcLog, "reading standard claims: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, oidcClaims{}, false
	}
	claims.Nonce = idToken.Nonce

	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		log.Error(oidcLog, "reading raw claims: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, oidcClaims{}, false
	}

	return rawClaims, claims, true
}

// completeOIDCLogin turns verified claims into a session.
func (app *ReactAppWrapper) completeOIDCLogin(c *gin.Context, rawClaims map[string]any, claims oidcClaims) {
	identity, err := resolveOIDCIdentity(app.cfg.OIDC, rawClaims, claims)
	if err != nil {
		switch {
		case errors.Is(err, errNoUserID):
			log.Warn(oidcLog, "no userid: the claim ", identity.ClaimName, " is absent or empty")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "userid claim not found or empty"})
		case errors.Is(err, errEmailNotVerified):
			log.Warn(oidcLog, "rejected login, email not verified: ", identity.Value)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "email not verified"})
		default:
			log.Error(oidcLog, "resolving identity: ", err)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	userKey := model.NormalizeUserID(identity.Value)
	if userKey == "" {
		log.Warn(oidcLog, "the claim ", identity.ClaimName, " holds nothing usable as a userid")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "userid claim not usable"})
		return
	}

	user, err := app.getOrProvisionUser(userKey, identity, claims, evaluateOIDCAdminStatus(app.cfg.OIDC, rawClaims))
	if err != nil {
		if errors.Is(err, errUserDisabled) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if _, err := app.issueWebSession(c, user); err != nil {
		log.Error(oidcLog, "issuing the session: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusFound, OIDCSuccessPath)
}

// getOrProvisionUser finds the account for a provider identity, creating it on
// first login. adminStatus is nil when admin is not claim driven, and in that
// case the stored flag is left exactly as an administrator set it.
func (app *ReactAppWrapper) getOrProvisionUser(userKey string, identity oidcUserIdentity, claims oidcClaims, adminStatus *bool) (*model.User, error) {
	isAdmin := adminStatus != nil && *adminStatus

	user, err := app.userStorer.GetUser(userKey)
	if err != nil {
		if !errors.Is(err, storage.ErrUserNotFound) {
			log.Error(oidcLog, "looking up ", userKey, ": ", err)
			return nil, err
		}
		newUser, err := app.provisionNewUser(userKey, claims, isAdmin)
		if err != nil {
			return nil, err
		}
		log.Info(oidcLog, "provisioned ", userKey, " from claim ", identity.ClaimName, ", admin=", isAdmin)
		return newUser, nil
	}

	if user.Disabled {
		log.Warn(oidcLog, "refused login for the disabled account ", userKey)
		return nil, errUserDisabled
	}

	// An account that already existed is adopted rather than created. Say so:
	// otherwise a local account quietly changing hands leaves no trace at all.
	log.Info(oidcLog, "signed in existing account ", userKey, " from claim ", identity.ClaimName)

	if adminStatus != nil && user.IsAdmin != *adminStatus {
		user.IsAdmin = *adminStatus
		if err := app.userStorer.UpdateUser(user); err != nil {
			log.Error(oidcLog, "updating admin status for ", userKey, ": ", err)
			return nil, err
		}
		log.Info(oidcLog, "admin status for ", userKey, " is now ", *adminStatus)
	}

	return user, nil
}

// provisionNewUser creates an account for a provider identity. The password is
// random and unusable on purpose: this account is reached through the provider.
func (app *ReactAppWrapper) provisionNewUser(userKey string, claims oidcClaims, isAdmin bool) (*model.User, error) {
	password, err := model.GenPassword()
	if err != nil {
		log.Error(oidcLog, "generating a placeholder password: ", err)
		return nil, err
	}

	user, err := model.NewUser(userKey, password)
	if err != nil {
		log.Error(oidcLog, "building the user: ", err)
		return nil, err
	}
	// model.NewUser runs the id through the legacy sanitizer, which would undo
	// the normalization the lookup key was built with.
	user.ID = userKey
	user.Email = userKey

	if email := strings.TrimSpace(claims.Email); email != "" {
		user.Email = strings.ToLower(email)
		user.EmailVerified = claimIsTrue(claims.EmailVerified)
	}
	if claims.Name != "" {
		user.Name = claims.Name
	}
	if claims.GivenName != "" {
		user.GivenName = claims.GivenName
	}
	if claims.FamilyName != "" {
		user.FamilyName = claims.FamilyName
	}
	if claims.PreferredUsername != "" {
		user.Nickname = claims.PreferredUsername
	}
	user.IsAdmin = isAdmin

	if err := app.userStorer.RegisterUser(user); err != nil {
		log.Error(oidcLog, "registering ", userKey, ": ", err)
		return nil, err
	}
	return user, nil
}

// meHandler returns the signed in user's profile. The session token is HttpOnly,
// so this is how the frontend learns who it is holding a session for.
func (app *ReactAppWrapper) meHandler(c *gin.Context) {
	user, err := app.userStorer.GetUser(userID(c))
	if err != nil {
		log.Error(uiLogger, "[me] ", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	scopes := ""
	if user.Sync15 {
		scopes = isSync15Key
	}
	roles := []string{"User"}
	if user.IsAdmin {
		roles = []string{AdminRole}
	}

	c.JSON(http.StatusOK, gin.H{
		"UserID": user.ID,
		"Email":  user.Email,
		"Scopes": scopes,
		"Roles":  roles,
	})
}
