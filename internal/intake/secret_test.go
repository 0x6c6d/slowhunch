package intake

import (
	"strings"
	"testing"
)

func TestSecretPatterns(t *testing.T) {
	positives := []string{
		"key: sk-" + strings.Repeat("a", 20),
		"id " + "AKIA" + strings.Repeat("A", 16) + " end",
		"-----BEGIN " + "RSA PRIVATE KEY-----\nabc",
		"-----BEGIN " + "PRIVATE KEY-----",
		"token " + "ghp_" + strings.Repeat("b", 36),
		"xoxb-" + strings.Repeat("1", 12),
		"xoxp-" + strings.Repeat("c", 10),
	}
	for _, p := range positives {
		if !ContainsSecret(p) {
			t.Errorf("ContainsSecret(%q) = false, want true", p)
		}
	}
	negatives := []string{
		"The trees are coming into leaf.",
		"the sk- prefix is used by some providers",
		"skip-to-content",
		"sk-short",
		"AKIA is a prefix",
		"ghp_tooShort",
		"xoxb-short",
		"-----BEGIN PUBLIC KEY-----",
		"",
	}
	for _, n := range negatives {
		if ContainsSecret(n) {
			t.Errorf("ContainsSecret(%q) = true, want false", n)
		}
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("auth test-key and test-key again", "test-key"); got != "auth *** and *** again" {
		t.Fatalf("got %q", got)
	}
	if got := Redact("a b c", "", "b"); got != "a *** c" {
		t.Fatalf("got %q", got)
	}
	if got := Redact("nothing here"); got != "nothing here" {
		t.Fatalf("got %q", got)
	}
	if got := Redact("x1 y2", "x1", "y2"); got != "*** ***" {
		t.Fatalf("got %q", got)
	}
}
