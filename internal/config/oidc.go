package config

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

const (
	// EnvOIDCProviderURL the issuer discovery url
	EnvOIDCProviderURL = "OIDC_PROVIDER_URL"
	// EnvOIDCClientID the oauth2 client id
	EnvOIDCClientID = "OIDC_CLIENT_ID"
	// EnvOIDCClientSecret the oauth2 client secret
	EnvOIDCClientSecret = "OIDC_CLIENT_SECRET"
	// EnvOIDCRedirectURL the callback url registered with the provider
	EnvOIDCRedirectURL = "OIDC_REDIRECT_URL"
	// EnvOIDCDisableLocalLogin turns the password form and registration off
	EnvOIDCDisableLocalLogin = "OIDC_DISABLE_LOCAL_LOGIN"
	// EnvOIDCUserIDClaim the claim used as the rmfakecloud user id
	EnvOIDCUserIDClaim = "OIDC_USERID_CLAIM"
	// EnvOIDCExtraScopes whitespace separated scopes added to the core three
	EnvOIDCExtraScopes = "OIDC_EXTRA_SCOPES"
	// EnvOIDCAdminClaim dotted path to the claim holding role values
	EnvOIDCAdminClaim = "OIDC_ADMIN_CLAIM"
	// EnvOIDCAdminClaimValue the value in that claim which grants admin
	EnvOIDCAdminClaimValue = "OIDC_ADMIN_CLAIM_VALUE"
	// EnvOIDCDisplayName the label on the login button
	EnvOIDCDisplayName = "OIDC_DISPLAY_NAME"
	// EnvOIDCAllowUnverifiedEmail lets an unverified email identity log in
	EnvOIDCAllowUnverifiedEmail = "OIDC_ALLOW_UNVERIFIED_EMAIL"
	// DefaultOIDCUserIDClaim the claim used when none is configured
	DefaultOIDCUserIDClaim = "preferred_username"
	// DefaultOIDCDisplayName the label used when none is configured
	DefaultOIDCDisplayName = "Login with OIDC"
)

// OIDCConfig holds the OpenID Connect settings. The zero value means OIDC is off.
type OIDCConfig struct {
	ProviderURL  string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// DisableLocalLogin removes the password form and the register endpoint.
	// It only has an effect while OIDC is enabled, otherwise the instance
	// would have no way in at all.
	DisableLocalLogin bool
	UserIDClaim       string
	ExtraScopes       []string
	AdminClaim        string
	AdminClaimValue   string
	DisplayName       string
	// AllowUnverifiedEmail drops the email_verified requirement. Without it,
	// a provider that lets a user set any email could be used to take over
	// an account belonging to that address.
	AllowUnverifiedEmail bool
}

// AdminFromClaim reports whether admin is decided by the provider. Both halves
// are needed: a claim path with nothing to match it against is not a rule.
func (o *OIDCConfig) AdminFromClaim() bool {
	return o.AdminClaim != "" && o.AdminClaimValue != ""
}

// Label is the text on the login button.
func (o *OIDCConfig) Label() string {
	if o.DisplayName == "" {
		return DefaultOIDCDisplayName
	}
	return o.DisplayName
}

// Enabled reports whether every required field is present.
func (o *OIDCConfig) Enabled() bool {
	return o.ProviderURL != "" && o.ClientID != "" &&
		o.ClientSecret != "" && o.RedirectURL != ""
}

// partiallyConfigured reports a half-filled config, which is always a mistake.
func (o *OIDCConfig) partiallyConfigured() bool {
	any := o.ProviderURL != "" || o.ClientID != "" ||
		o.ClientSecret != "" || o.RedirectURL != ""
	return any && !o.Enabled()
}

// LocalLoginEnabled reports whether the password form and register endpoint stay live.
func (o *OIDCConfig) LocalLoginEnabled() bool {
	return !o.Enabled() || !o.DisableLocalLogin
}

// Scopes is the scope list sent on the authorization request.
func (o *OIDCConfig) Scopes() []string {
	scopes := []string{"openid", "email", "profile"}
	for _, s := range o.ExtraScopes {
		if !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

func oidcFromEnv() OIDCConfig {
	userIDClaim := os.Getenv(EnvOIDCUserIDClaim)
	if userIDClaim == "" {
		userIDClaim = DefaultOIDCUserIDClaim
	}

	disableLocalLogin, _ := strconv.ParseBool(os.Getenv(EnvOIDCDisableLocalLogin))
	allowUnverifiedEmail, _ := strconv.ParseBool(os.Getenv(EnvOIDCAllowUnverifiedEmail))

	return OIDCConfig{
		ProviderURL:       os.Getenv(EnvOIDCProviderURL),
		ClientID:          os.Getenv(EnvOIDCClientID),
		ClientSecret:      os.Getenv(EnvOIDCClientSecret),
		RedirectURL:       os.Getenv(EnvOIDCRedirectURL),
		DisableLocalLogin: disableLocalLogin,
		UserIDClaim:       userIDClaim,
		ExtraScopes:       strings.Fields(os.Getenv(EnvOIDCExtraScopes)),
		AdminClaim:        os.Getenv(EnvOIDCAdminClaim),
		AdminClaimValue:   os.Getenv(EnvOIDCAdminClaimValue),
		DisplayName:       os.Getenv(EnvOIDCDisplayName),

		AllowUnverifiedEmail: allowUnverifiedEmail,
	}
}

// validateOIDC checks the OIDC settings as a whole. An instance with no OIDC
// configured is always valid; a half-configured one never is.
func (cfg *Config) validateOIDC() error {
	if cfg.OIDC.partiallyConfigured() {
		return fmt.Errorf("OIDC is partially configured; set all of %s, %s, %s, %s",
			EnvOIDCProviderURL, EnvOIDCClientID, EnvOIDCClientSecret, EnvOIDCRedirectURL)
	}
	if !cfg.OIDC.Enabled() {
		return nil
	}

	// The state, nonce and PKCE cookies carry the whole flow. Over plain http
	// they are readable, which is the one thing the flow assumes they are not.
	if !cfg.HTTPSCookie {
		return fmt.Errorf("OIDC requires secure cookies; set %s=true", envHTTPSCookie)
	}

	u, err := url.Parse(cfg.OIDC.RedirectURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s %q cannot be parsed or is missing an http/https scheme",
			EnvOIDCRedirectURL, cfg.OIDC.RedirectURL)
	}

	return nil
}

// OIDCCallbackPath is the path the provider must be told to redirect back to.
const OIDCCallbackPath = "/ui/api/oidc/callback"
