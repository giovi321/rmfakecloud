package config

import (
	"strings"
	"testing"
)

func fullOIDC() OIDCConfig {
	return OIDCConfig{
		ProviderURL:  "https://sso.example.com",
		ClientID:     "rmfakecloud",
		ClientSecret: "secret",
		RedirectURL:  "https://rm.example.com/ui/api/oidc/callback",
	}
}

func TestOIDCEnabledNeedsAllFourFields(t *testing.T) {
	full := fullOIDC()
	if !full.Enabled() {
		t.Error("a fully populated config should be enabled")
	}

	for _, tc := range []struct {
		name  string
		blank func(*OIDCConfig)
	}{
		{"no provider url", func(o *OIDCConfig) { o.ProviderURL = "" }},
		{"no client id", func(o *OIDCConfig) { o.ClientID = "" }},
		{"no client secret", func(o *OIDCConfig) { o.ClientSecret = "" }},
		{"no redirect url", func(o *OIDCConfig) { o.RedirectURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := fullOIDC()
			tc.blank(&o)
			if o.Enabled() {
				t.Error("should not be enabled")
			}
		})
	}
}

func TestOIDCZeroValueIsDisabledAndNotPartial(t *testing.T) {
	var o OIDCConfig
	if o.Enabled() {
		t.Error("the zero value must be disabled")
	}
	if o.partiallyConfigured() {
		t.Error("the zero value must not count as partially configured")
	}
}

func TestOIDCPartiallyConfiguredDetectsAnyFieldWithoutTheRest(t *testing.T) {
	var o OIDCConfig
	o.ClientID = "rmfakecloud"
	if !o.partiallyConfigured() {
		t.Error("one field set out of four is partial configuration")
	}

	full := fullOIDC()
	if full.partiallyConfigured() {
		t.Error("a complete config is not partial")
	}
}

func TestOIDCScopesAlwaysCarryTheCoreThreeAndDeduplicate(t *testing.T) {
	o := fullOIDC()
	o.ExtraScopes = []string{"groups", "email", "groups"}

	got := o.Scopes()
	want := []string{"openid", "email", "profile", "groups"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestOIDCFromEnvAppliesTheUserIDClaimDefault(t *testing.T) {
	t.Setenv(EnvOIDCUserIDClaim, "")

	o := oidcFromEnv()

	if o.UserIDClaim != DefaultOIDCUserIDClaim {
		t.Errorf("got %q, want %q", o.UserIDClaim, DefaultOIDCUserIDClaim)
	}
}

// The upstream pull request this is based on documented OIDC_DISABLE_LOCAL_LOGIN
// with a default of false and then never read it, which made the behaviour
// unconditional. These two tests exist so that cannot happen again.
func TestOIDCFromEnvLeavesLocalLoginOnByDefault(t *testing.T) {
	t.Setenv(EnvOIDCDisableLocalLogin, "")

	if oidcFromEnv().DisableLocalLogin {
		t.Error("local login must stay available unless explicitly disabled")
	}
}

func TestOIDCFromEnvReadsDisableLocalLogin(t *testing.T) {
	for _, raw := range []string{"true", "TRUE", "1"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(EnvOIDCDisableLocalLogin, raw)

			if !oidcFromEnv().DisableLocalLogin {
				t.Errorf("%s=%q must disable local login", EnvOIDCDisableLocalLogin, raw)
			}
		})
	}
}

func TestOIDCFromEnvSplitsExtraScopesOnWhitespace(t *testing.T) {
	t.Setenv(EnvOIDCExtraScopes, "  groups   roles ")

	got := oidcFromEnv().ExtraScopes
	want := []string{"groups", "roles"}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLocalLoginAvailableWhenOIDCIsOff(t *testing.T) {
	var o OIDCConfig
	o.DisableLocalLogin = true

	if !o.LocalLoginEnabled() {
		t.Error("disabling local login must have no effect while OIDC is off, or the instance has no way in")
	}
}

func TestLocalLoginCoexistsWithOIDCByDefault(t *testing.T) {
	o := fullOIDC()

	if !o.LocalLoginEnabled() {
		t.Error("enabling OIDC alone must not remove password login")
	}
}

func TestLocalLoginOffWhenOIDCEnabledAndExplicitlyDisabled(t *testing.T) {
	o := fullOIDC()
	o.DisableLocalLogin = true

	if o.LocalLoginEnabled() {
		t.Error("local login should be off")
	}
}

func TestValidateOIDCRejectsAPartialConfig(t *testing.T) {
	cfg := Config{HTTPSCookie: true}
	cfg.OIDC.ClientID = "rmfakecloud"

	if err := cfg.validateOIDC(); err == nil {
		t.Error("a partial config must be rejected, not silently ignored")
	}
}

func TestValidateOIDCRejectsInsecureCookies(t *testing.T) {
	cfg := Config{OIDC: fullOIDC(), HTTPSCookie: false}

	if err := cfg.validateOIDC(); err == nil {
		t.Error("OIDC flow cookies must not travel over plain http")
	}
}

func TestValidateOIDCRejectsARedirectURLWithoutAScheme(t *testing.T) {
	cfg := Config{OIDC: fullOIDC(), HTTPSCookie: true}
	cfg.OIDC.RedirectURL = "rm.example.com/ui/api/oidc/callback"

	if err := cfg.validateOIDC(); err == nil {
		t.Error("a redirect url with no http/https scheme must be rejected")
	}
}

func TestValidateOIDCAcceptsACompleteConfig(t *testing.T) {
	cfg := Config{OIDC: fullOIDC(), HTTPSCookie: true}

	if err := cfg.validateOIDC(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateOIDCAcceptsAnInstanceWithNoOIDCAtAll(t *testing.T) {
	cfg := Config{HTTPSCookie: false}

	if err := cfg.validateOIDC(); err != nil {
		t.Errorf("an instance without OIDC must not be held to OIDC rules: %v", err)
	}
}

func TestFromEnvCarriesTheOIDCSettings(t *testing.T) {
	t.Setenv(EnvOIDCProviderURL, "https://sso.example.com")
	t.Setenv(EnvOIDCClientID, "rmfakecloud")
	t.Setenv(EnvOIDCClientSecret, "secret")
	t.Setenv(EnvOIDCRedirectURL, "https://rm.example.com/ui/api/oidc/callback")
	t.Setenv(EnvOIDCDisableLocalLogin, "true")

	cfg := FromEnv()

	if !cfg.OIDC.Enabled() {
		t.Fatal("FromEnv did not carry the OIDC settings onto Config")
	}
	if cfg.OIDC.LocalLoginEnabled() {
		t.Error("FromEnv did not carry DisableLocalLogin")
	}
}

func TestEnvVarsDocumentsEveryOIDCVariable(t *testing.T) {
	usage := EnvVars()

	for _, name := range []string{
		EnvOIDCProviderURL, EnvOIDCClientID, EnvOIDCClientSecret,
		EnvOIDCRedirectURL, EnvOIDCDisableLocalLogin, EnvOIDCUserIDClaim,
		EnvOIDCExtraScopes,
	} {
		if !strings.Contains(usage, name) {
			t.Errorf("%s is missing from the usage text", name)
		}
	}

	if strings.Contains(usage, "%!s(MISSING)") || strings.Contains(usage, "%!(EXTRA") {
		t.Error("the usage format string and its arguments are out of step")
	}
}

func TestOIDCFromEnvReadsTheAdminClaimSettings(t *testing.T) {
	t.Setenv(EnvOIDCAdminClaim, "realm_access.roles")
	t.Setenv(EnvOIDCAdminClaimValue, "rmfakecloud-admins")

	o := oidcFromEnv()

	if o.AdminClaim != "realm_access.roles" {
		t.Errorf("AdminClaim: got %q", o.AdminClaim)
	}
	if o.AdminClaimValue != "rmfakecloud-admins" {
		t.Errorf("AdminClaimValue: got %q", o.AdminClaimValue)
	}
}

func TestAdminFromClaimNeedsBothHalves(t *testing.T) {
	o := fullOIDC()
	if o.AdminFromClaim() {
		t.Error("neither half set: admin must not be claim driven")
	}

	o.AdminClaim = "groups"
	if o.AdminFromClaim() {
		t.Error("a claim with no value to match is not a rule")
	}

	o.AdminClaimValue = "admins"
	if !o.AdminFromClaim() {
		t.Error("both halves set: admin should be claim driven")
	}
}

func TestOIDCDisplayNameFallsBackToADefaultLabel(t *testing.T) {
	t.Setenv(EnvOIDCDisplayName, "")
	o := oidcFromEnv()
	if got := o.Label(); got != DefaultOIDCDisplayName {
		t.Errorf("got %q, want %q", got, DefaultOIDCDisplayName)
	}

	t.Setenv(EnvOIDCDisplayName, "Login with Authelia")
	o = oidcFromEnv()
	if got := o.Label(); got != "Login with Authelia" {
		t.Errorf("got %q", got)
	}
}

func TestOIDCFromEnvReadsAllowUnverifiedEmail(t *testing.T) {
	t.Setenv(EnvOIDCAllowUnverifiedEmail, "")
	if oidcFromEnv().AllowUnverifiedEmail {
		t.Error("an unverified email must be rejected by default")
	}

	t.Setenv(EnvOIDCAllowUnverifiedEmail, "true")
	if !oidcFromEnv().AllowUnverifiedEmail {
		t.Error("the opt-out was not read")
	}
}
