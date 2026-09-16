package ui

import (
	"errors"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/config"
)

func oidcCfg() config.OIDCConfig {
	return config.OIDCConfig{
		ProviderURL:  "https://sso.example.com",
		ClientID:     "rmfakecloud",
		ClientSecret: "secret",
		RedirectURL:  "https://rm.example.com" + config.OIDCCallbackPath,
		UserIDClaim:  config.DefaultOIDCUserIDClaim,
	}
}

func TestExtractClaimPathFindsATopLevelClaim(t *testing.T) {
	raw := map[string]any{"preferred_username": "alice"}

	got, ok := extractClaimPath(raw, "preferred_username")

	if !ok || got != "alice" {
		t.Errorf("got %v, %v", got, ok)
	}
}

func TestExtractClaimPathWalksADottedPath(t *testing.T) {
	raw := map[string]any{
		"realm_access": map[string]any{"roles": []any{"admin", "user"}},
	}

	got, ok := extractClaimPath(raw, "realm_access.roles")

	if !ok {
		t.Fatal("path not found")
	}
	roles, isSlice := got.([]any)
	if !isSlice || len(roles) != 2 || roles[0] != "admin" {
		t.Errorf("got %v", got)
	}
}

func TestExtractClaimPathReportsAMissingKey(t *testing.T) {
	raw := map[string]any{"sub": "alice"}

	if _, ok := extractClaimPath(raw, "groups"); ok {
		t.Error("a missing key must not report found")
	}
	if _, ok := extractClaimPath(raw, "sub.nested"); ok {
		t.Error("descending into a non-map must not report found")
	}
}

func TestClaimHasValueAcrossTheShapesProvidersUse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  bool
	}{
		{"string match", "admins", true},
		{"string mismatch", "users", false},
		{"any slice match", []any{"users", "admins"}, true},
		{"any slice mismatch", []any{"users"}, false},
		{"string slice match", []string{"admins"}, true},
		{"slice of non strings", []any{1, 2}, false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := claimHasValue(tc.value, "admins"); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClaimIsTrueAcceptsBoolAndStringForms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string TRUE", "TRUE", true},
		{"string padded", " true ", true},
		{"string false", "false", false},
		{"absent", nil, false},
		{"number", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := claimIsTrue(tc.value); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveIdentityUsesTheConfiguredClaim(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"preferred_username": "alice", "email": "alice@example.com"}
	claims := oidcClaims{PreferredUsername: "alice", Email: "alice@example.com"}

	identity, err := resolveOIDCIdentity(cfg, raw, claims)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Value != "alice" {
		t.Errorf("got %q, want %q", identity.Value, "alice")
	}
	if identity.usesEmail() {
		t.Error("the identity came from preferred_username, not email")
	}
}

func TestResolveIdentityDoesNotDemandEmailVerificationForANonEmailClaim(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"preferred_username": "alice"}
	claims := oidcClaims{PreferredUsername: "alice", Email: "alice@example.com", EmailVerified: false}

	if _, err := resolveOIDCIdentity(cfg, raw, claims); err != nil {
		t.Errorf("a username identity does not depend on the email being verified: %v", err)
	}
}

func TestResolveIdentityFallsBackToEmailWhenTheClaimIsAbsent(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"email": "alice@example.com"}
	claims := oidcClaims{Email: "alice@example.com", EmailVerified: true}

	identity, err := resolveOIDCIdentity(cfg, raw, claims)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !identity.usesEmail() || identity.Value != "alice@example.com" {
		t.Errorf("got %+v", identity)
	}
}

func TestResolveIdentityLowercasesAnEmailIdentity(t *testing.T) {
	cfg := oidcCfg()
	cfg.UserIDClaim = "email"
	raw := map[string]any{"email": "Alice@Example.COM"}
	claims := oidcClaims{Email: "Alice@Example.COM", EmailVerified: true}

	identity, err := resolveOIDCIdentity(cfg, raw, claims)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Value != "alice@example.com" {
		t.Errorf("got %q, repeated logins would resolve to different accounts", identity.Value)
	}
}

func TestResolveIdentityRejectsAnUnverifiedEmail(t *testing.T) {
	cfg := oidcCfg()
	cfg.UserIDClaim = "email"
	raw := map[string]any{"email": "alice@example.com"}
	claims := oidcClaims{Email: "alice@example.com", EmailVerified: false}

	_, err := resolveOIDCIdentity(cfg, raw, claims)

	if !errors.Is(err, errEmailNotVerified) {
		t.Errorf("got %v, want errEmailNotVerified", err)
	}
}

func TestResolveIdentityRejectsAnUnverifiedEmailReachedByFallback(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"email": "alice@example.com"}
	claims := oidcClaims{Email: "alice@example.com", EmailVerified: false}

	_, err := resolveOIDCIdentity(cfg, raw, claims)

	if !errors.Is(err, errEmailNotVerified) {
		t.Errorf("the fallback path must enforce verification too, got %v", err)
	}
}

func TestResolveIdentityAllowsAnUnverifiedEmailWhenOptedIn(t *testing.T) {
	cfg := oidcCfg()
	cfg.UserIDClaim = "email"
	cfg.AllowUnverifiedEmail = true
	raw := map[string]any{"email": "alice@example.com"}
	claims := oidcClaims{Email: "alice@example.com", EmailVerified: false}

	if _, err := resolveOIDCIdentity(cfg, raw, claims); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveIdentityFailsWhenNothingIdentifiesTheUser(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"sub": "1234"}

	_, err := resolveOIDCIdentity(cfg, raw, oidcClaims{})

	if !errors.Is(err, errNoUserID) {
		t.Errorf("got %v, want errNoUserID", err)
	}
}

func TestResolveIdentitySkipsABlankConfiguredClaim(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"preferred_username": "   ", "email": "alice@example.com"}
	claims := oidcClaims{Email: "alice@example.com", EmailVerified: true}

	identity, err := resolveOIDCIdentity(cfg, raw, claims)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !identity.usesEmail() {
		t.Errorf("a whitespace only claim is not an identity, got %+v", identity)
	}
}

func TestAdminStatusIsUndecidedWhenNoClaimIsConfigured(t *testing.T) {
	cfg := oidcCfg()
	raw := map[string]any{"groups": []any{"admins"}}

	if got := evaluateOIDCAdminStatus(cfg, raw); got != nil {
		t.Errorf("got %v, want nil so an existing admin is left alone", *got)
	}
}

func TestAdminStatusFollowsTheConfiguredClaim(t *testing.T) {
	cfg := oidcCfg()
	cfg.AdminClaim = "groups"
	cfg.AdminClaimValue = "rmfakecloud-admins"

	for _, tc := range []struct {
		name string
		raw  map[string]any
		want bool
	}{
		{"member", map[string]any{"groups": []any{"users", "rmfakecloud-admins"}}, true},
		{"not a member", map[string]any{"groups": []any{"users"}}, false},
		{"claim absent", map[string]any{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateOIDCAdminStatus(cfg, tc.raw)
			if got == nil {
				t.Fatal("got nil, want a decided value")
			}
			if *got != tc.want {
				t.Errorf("got %v, want %v", *got, tc.want)
			}
		})
	}
}

func TestAdminStatusReadsADottedClaimPath(t *testing.T) {
	cfg := oidcCfg()
	cfg.AdminClaim = "realm_access.roles"
	cfg.AdminClaimValue = "admin"
	raw := map[string]any{
		"realm_access": map[string]any{"roles": []any{"admin"}},
	}

	got := evaluateOIDCAdminStatus(cfg, raw)

	if got == nil || !*got {
		t.Error("a dotted admin claim path was not resolved")
	}
}
