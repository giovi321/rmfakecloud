package ui

import (
	"errors"
	"strings"

	"github.com/ddvk/rmfakecloud/internal/config"
)

var (
	errNoUserID         = errors.New("no userid available: the configured claim is absent or empty")
	errEmailNotVerified = errors.New("email not verified")
)

// oidcClaims are the standard claims read off the ID token. The configurable
// userid and admin claims may be arbitrary dotted paths, so those are read from
// the raw map instead.
type oidcClaims struct {
	Nonce string `json:"nonce"`
	Email string `json:"email"`
	// EmailVerified arrives as a bool from most providers and as a string from some.
	EmailVerified     any    `json:"email_verified"`
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	PreferredUsername string `json:"preferred_username"`
}

// oidcUserIdentity is the value that becomes the rmfakecloud user id, together
// with the claim it came from.
type oidcUserIdentity struct {
	Value     string
	ClaimName string
}

func newOIDCUserIdentity(value, claimName string) oidcUserIdentity {
	if claimName == "email" {
		// Email identities compare case insensitively, otherwise the same person
		// logging in twice can land on two different accounts.
		value = strings.ToLower(value)
	}
	return oidcUserIdentity{Value: value, ClaimName: claimName}
}

func (identity oidcUserIdentity) usesEmail() bool { return identity.ClaimName == "email" }

// extractClaimPath walks a dotted path such as "realm_access.roles".
func extractClaimPath(raw map[string]any, path string) (any, bool) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := raw[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return extractClaimPath(nested, parts[1])
}

// claimHasValue reports whether a claim contains expected. Providers send role
// claims as a bare string or as a list, so both shapes are accepted.
func claimHasValue(value any, expected string) bool {
	switch v := value.(type) {
	case string:
		return v == expected
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == expected {
				return true
			}
		}
	}
	return false
}

// claimIsTrue reads a boolean claim that may arrive as a bool or as a string.
func claimIsTrue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return false
}

// resolveOIDCIdentity picks the value that becomes the user id. The configured
// claim wins; if it yields nothing the email claim is used instead. Email
// identities must be verified, since a provider that lets a user set any address
// would otherwise hand them somebody else's account.
func resolveOIDCIdentity(cfg config.OIDCConfig, raw map[string]any, claims oidcClaims) (oidcUserIdentity, error) {
	claimName := cfg.UserIDClaim
	if claimName == "" {
		claimName = config.DefaultOIDCUserIDClaim
	}

	if claimVal, ok := extractClaimPath(raw, claimName); ok {
		if strVal, ok := claimVal.(string); ok {
			if value := strings.TrimSpace(strVal); value != "" {
				identity := newOIDCUserIdentity(value, claimName)
				return identity, validateEmailIdentity(cfg, identity, claims)
			}
		}
	}

	if claimName != "email" {
		if value := strings.TrimSpace(claims.Email); value != "" {
			identity := newOIDCUserIdentity(value, "email")
			return identity, validateEmailIdentity(cfg, identity, claims)
		}
	}

	return oidcUserIdentity{ClaimName: claimName}, errNoUserID
}

func validateEmailIdentity(cfg config.OIDCConfig, identity oidcUserIdentity, claims oidcClaims) error {
	if identity.usesEmail() && !cfg.AllowUnverifiedEmail && !claimIsTrue(claims.EmailVerified) {
		return errEmailNotVerified
	}
	return nil
}

// evaluateOIDCAdminStatus returns nil when admin is not claim driven, so callers
// leave an existing user's admin flag alone. A non-nil result is the decision,
// re-made on every login.
func evaluateOIDCAdminStatus(cfg config.OIDCConfig, raw map[string]any) *bool {
	if !cfg.AdminFromClaim() {
		return nil
	}
	isAdmin := false
	if claimVal, ok := extractClaimPath(raw, cfg.AdminClaim); ok {
		isAdmin = claimHasValue(claimVal, cfg.AdminClaimValue)
	}
	return &isAdmin
}
