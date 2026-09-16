package ui

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	backendVersionKey string = "BackendVersion"
)

// parseWebSessionCookie reads the auth cookie and returns the verified claims.
func (app *ReactAppWrapper) parseWebSessionCookie(c *gin.Context) (*WebUserClaims, error) {
	token, err := c.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	claims := &WebUserClaims{}
	if err := common.ClaimsFromToken(claims, token, app.cfg.JWTSecretKey); err != nil {
		return nil, err
	}
	if !slices.Contains(claims.Audience, WebUsage) {
		return nil, errors.New("wrong token audience")
	}
	return claims, nil
}

// webAuthenticated reports whether the request already carries a session, so a
// redirect decision can be made without aborting the request.
func (app *ReactAppWrapper) webAuthenticated(c *gin.Context) bool {
	_, err := app.parseWebSessionCookie(c)
	return err == nil
}

// IsAdmin checks if admin
func IsAdmin(c *gin.Context) bool {
	return c.GetBool(AdminRole)
}

func (app *ReactAppWrapper) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAdmin(c) {
			log.Warn("not admin")
			c.AbortWithStatus(http.StatusForbidden)
		}
	}
}

func (app *ReactAppWrapper) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err == http.ErrNoCookie {
			log.Warn("missing cookie, trying headers")
			token, err = common.GetToken(c)
		}

		if err != nil {
			log.Warn("[ui-authmiddleware] token parsing, ", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or incorrect token"})
			return
		}
		claims := &WebUserClaims{}
		err = common.ClaimsFromToken(claims, token, app.cfg.JWTSecretKey)
		if err != nil {
			log.Warn("[ui-authmiddleware] token verification, ", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or incorrect token"})
			return
		}

		if !slices.Contains(claims.Audience, WebUsage) {
			log.Warn("wrong token audience: ", claims.Audience)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or incorrect token"})
			return
		}

		scopes := strings.Fields(claims.Scopes)
		c.Set(backendVersionKey, common.Sync10)
		for _, s := range scopes {
			switch s {
			case isSync15Key:
				c.Set(backendVersionKey, common.Sync15)
				break
			}
		}

		uid := common.SanitizeUid(claims.UserID)

		// Revocation has to bite before the session expires, so the stored
		// record is consulted on every request rather than trusted from the
		// token. This costs one read per request and is the point of the flag.
		user, err := app.userStorer.GetUser(uid)
		if err != nil {
			log.Warn("[ui-authmiddleware] no such user: ", uid)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or incorrect token"})
			return
		}
		if user.Disabled {
			log.Warn("[ui-authmiddleware] disabled account: ", uid)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			return
		}

		c.Set(userIDContextKey, uid)

		brid := claims.BrowserID
		c.Set(browserIDContextKey, brid)
		for _, r := range claims.Roles {
			if r == AdminRole {
				c.Set(AdminRole, true)
				break
			}
		}
		log.Info("[ui-authmiddleware] User from token: ", uid)
		c.Next()
	}
}
