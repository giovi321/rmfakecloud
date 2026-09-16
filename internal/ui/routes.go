package ui

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// RegisterRoutes the apps routes
func (app *ReactAppWrapper) RegisterRoutes(router *gin.Engine) {
	router.StaticFS(app.prefix, app)

	router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("/favicon.ico", app.fs)
	})
	router.GET("/robots.txt", func(c *gin.Context) {
		c.FileFromFS("/robots.txt", app.fs)
	})

	//hack for index.html
	router.NoRoute(func(c *gin.Context) {
		uri := c.Request.RequestURI
		log.Info(uri)
		if strings.HasPrefix(uri, "/api") ||
			strings.HasPrefix(uri, "/ui/api") ||
			c.Request.Method != http.MethodGet {

			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// With local login disabled the provider is the only way in, so an
		// unauthenticated browser is sent straight there. The callback landing
		// page and the logout page are excluded: redirecting either one would
		// loop, and redirecting the logout page would sign the user back in.
		if !app.cfg.OIDC.LocalLoginEnabled() &&
			!strings.HasPrefix(uri, OIDCSuccessPath) &&
			!strings.HasPrefix(uri, OIDCLoggedOutPath) &&
			!app.webAuthenticated(c) {

			c.Redirect(http.StatusFound, OIDCLoginPath)
			return
		}

		app.serveIndex(c)
	})

	r := router.Group("/ui/api")
	// Served whether or not OIDC is configured: the frontend is compiled into
	// the binary and has no other way to find out what the login page should show.
	r.GET("oidc/info", app.oidcInfo)
	if app.cfg.OIDC.Enabled() {
		r.GET("oidc/login", app.oidcBegin)
		r.GET("oidc/callback", app.oidcCallback)
	}
	if app.cfg.OIDC.LocalLoginEnabled() {
		r.POST("register", app.register)
		r.POST("login", app.login)
	}
	r.GET("logout", func(c *gin.Context) {
		app.clearWebSession(c)
		c.Status(http.StatusOK)
	})
	//with authentication
	auth := r.Group("")
	auth.Use(app.authMiddleware())
	auth.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	auth.GET("me", app.meHandler)
	auth.GET("sync", func(c *gin.Context) {
		uid := userID(c)
		br := c.GetString(browserIDContextKey)
		log.Info("browser", br)
		app.h.NotifySync(uid, br)
	})

	auth.GET("newcode", app.newCode)

	// passcode (PIN) reset approval
	auth.GET("passcode/resets", app.listPasscodeResets)
	auth.POST("passcode/resets/:uuid/approve", app.approvePasscodeReset)
	auth.DELETE("passcode/resets/:uuid", app.dismissPasscodeReset)

	// auth.GET("profile", app.newCode)
	auth.POST("profile", app.changePassword)
	// auth.POST("changeEmail", app.changePassword)

	auth.GET("documents", app.listDocuments)
	auth.GET("documents/:docid", app.getDocument)
	auth.POST("documents/upload", app.createDocument)

	//move, rename
	auth.DELETE("documents/:docid", app.deleteDocument)
	auth.PUT("documents", app.updateDocument)
	auth.POST("folders", app.createFolder)
	auth.GET("documents/:docid/metadata", app.getDocumentMetadata)

	// integrations
	auth.GET("integrations", app.listIntegrations)
	auth.POST("integrations", app.createIntegration)
	auth.GET("integrations/:intid", app.getIntegration)
	auth.PUT("integrations/:intid", app.updateIntegration)
	auth.DELETE("integrations/:intid", app.deleteIntegration)

	auth.GET("integrations/:intid/explore/*path", app.exploreIntegration)
	auth.GET("integrations/:intid/metadata/*path", app.getMetadataIntegration)
	auth.GET("integrations/:intid/download/*path", app.downloadThroughIntegration)

	ss := auth.Group("screenshare")
	ss.GET("room", app.screenshareJoinActive)
	ss.GET("room/:roomId", app.screenshareGetRoom)
	ss.GET("offer", app.screenshareGetOffer)
	ss.POST("room/:roomId/answer", app.screenshareSendAnswer)
	ss.DELETE("room/:roomId", app.screenshareDeleteRoom)

	//admin
	admin := auth.Group("")
	admin.Use(app.adminMiddleware())
	admin.GET("users/:userid", app.getUser)
	admin.DELETE("users/:userid", app.deleteUser)
	admin.PUT("users", app.updateUser)
	admin.POST("users", app.createUser)
	admin.GET("users", app.getAppUsers)

	// Page templates. A template has to be installed on the tablet as well:
	// the device never asks the server for one, it only reports which it drew.
	admin.GET("templates", app.listTemplates)
	admin.POST("templates", app.createTemplate)
	admin.DELETE("templates/:"+templateNameParam, app.deleteTemplate)
}
