package intake

import (
	"regexp"
	"strings"
)

var secretRes = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
	regexp.MustCompile(`xox[bp]-[A-Za-z0-9-]{10,}`),
}

// ContainsSecret reports whether text looks like it contains an API key,
// access key, token or private key.
func ContainsSecret(text string) bool {
	for _, re := range secretRes {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// Redact replaces every occurrence of each non-empty secret in s with "***".
func Redact(s string, secrets ...string) string {
	for _, sec := range secrets {
		if sec != "" {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	return s
}
