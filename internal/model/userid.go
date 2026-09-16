package model

import (
	"regexp"
	"strings"
)

// userIDDisallowed matches everything a user id may not contain. Unlike the
// legacy sanitizer it lists the hyphen as a literal rather than leaving it to be
// read as a range, so a provider username like "jean-luc" keeps its shape
// instead of collapsing onto a different account.
var userIDDisallowed = regexp.MustCompile(`[^a-z0-9.@_-]+`)

// NormalizeUserID is the canonical key for an identity supplied by an OIDC
// provider: lowercased, trimmed, and reduced to the characters a user directory
// may safely be named with. It is deliberately not applied to the existing
// password login path, where directories are already named by sanitizeEmail and
// lowercasing would stop an existing account resolving.
//
// An empty result means the input carried no usable name, and the caller must
// reject the login rather than invent one.
func NormalizeUserID(id string) string {
	cleaned := userIDDisallowed.ReplaceAllString(strings.ToLower(strings.TrimSpace(id)), "")
	if strings.Trim(cleaned, ".") == "" {
		return ""
	}
	return cleaned
}
