package model

import "testing"

// The legacy sanitizer's character class is "[^a-zA-Z0-9.@-_]+", in which "@-_"
// is a range rather than three literals. These cases pin what it actually does,
// so a later change to it is a visible decision rather than a surprise. Existing
// user directories are named by it, so it is deliberately left alone.
func TestLegacySanitizerKeepsItsQuirks(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Jean-Luc", "JeanLuc"},
		{"ALICE@Example.com", "ALICE@Example.com"},
		{"a b", "ab"},
	} {
		if got := sanitizeEmail(tc.in); got != tc.want {
			t.Errorf("sanitizeEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeUserIDLowercasesAndTrims(t *testing.T) {
	if got := NormalizeUserID("  ALICE@Example.COM  "); got != "alice@example.com" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeUserIDKeepsTheHyphen(t *testing.T) {
	if got := NormalizeUserID("jean-luc"); got != "jean-luc" {
		t.Errorf("got %q, a provider username must not collide with a different account", got)
	}
}

func TestNormalizeUserIDStripsPathAndShellCharacters(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`alice\bob`, "alicebob"},
		{"alice/bob", "alicebob"},
		{"alice[bob]", "alicebob"},
		{"alice^bob", "alicebob"},
		{"alice bob", "alicebob"},
		{"alice:bob", "alicebob"},
	} {
		if got := NormalizeUserID(tc.in); got != tc.want {
			t.Errorf("NormalizeUserID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeUserIDIsIdempotent(t *testing.T) {
	once := NormalizeUserID("Jean-Luc Picard@Example.COM")
	if twice := NormalizeUserID(once); twice != once {
		t.Errorf("%q then %q", once, twice)
	}
}

func TestNormalizeUserIDRefusesAnIDThatIsNotAName(t *testing.T) {
	for _, in := range []string{"", "   ", "..", "...", "/", `\`, "./.."} {
		if got := NormalizeUserID(in); got != "" {
			t.Errorf("NormalizeUserID(%q) = %q, want an empty result the caller can reject", in, got)
		}
	}
}
