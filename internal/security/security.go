// Package security holds helpers that keep verification production-safe:
// secret redaction, safe temporary keys, and input validation.
package security

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
)

var (
	passwordInURL = regexp.MustCompile(`(://[^:/@]+:)[^@]+@`)
	passwordInDSN = regexp.MustCompile(`(?i)(password|pwd)=[^;\s]+`)
)

// Redact removes credentials from a string before it is logged or reported.
// It handles postgres/mysql DSNs (user:pass@host and password=... forms)
// and generic URLs. It never returns the secret itself.
func Redact(s string) string {
	if s == "" {
		return s
	}
	s = passwordInURL.ReplaceAllString(s, `${1}***@`)
	s = passwordInDSN.ReplaceAllString(s, `${1}=***`)
	return s
}

// RedactURL redacts the password component of a URL, if any.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = url.UserPassword(u.User.Username(), "***")
	return u.String()
}

// RandomToken returns n random bytes as hex, for namespaced probe keys.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// rand.Read failing is effectively impossible on a healthy system;
		// fall back to a fixed-length token rather than crashing a check.
		return "shipcheck-fallback"
	}
	return hex.EncodeToString(b)
}

// ProbeKey returns a uniquely namespaced, temporary Redis key that can
// never collide with application keys. Callers must delete it after use
// (and set a short TTL as a second safety net).
func ProbeKey() string {
	return "shipcheck:probe:" + RandomToken(8)
}

// IsSafeName rejects names that could be used for path traversal or
// shell injection in file/process checks. Names are identifiers, not paths.
func IsSafeName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	if strings.ContainsAny(s, "/\\;|&$`\"' \t\n\r") {
		return false
	}
	return true
}
