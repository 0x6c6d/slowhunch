package intake

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

const delimPrefix = "SLOWHUNCH-INTAKE-"

var lookAlikeRe = regexp.MustCompile(`(?i)SLOWHUNCH-INTAKE`)

// NewDelimiter returns a random fence delimiter ("SLOWHUNCH-INTAKE-" plus 16
// lowercase hex characters) that does not occur in text, ignoring case.
func NewDelimiter(text string) (string, error) {
	upper := strings.ToUpper(text)
	for {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		d := delimPrefix + hex.EncodeToString(b[:])
		if !strings.Contains(upper, strings.ToUpper(d)) {
			return d, nil
		}
	}
}

// Neutralize rewrites text so that it can neither open nor close a fence:
// runs of "<<<" and ">>>" are broken up with spaces and every
// case-insensitive "SLOWHUNCH-INTAKE" becomes "SLOWHUNCH_INTAKE".
func Neutralize(text string) string {
	for strings.Contains(text, "<<<") {
		text = strings.ReplaceAll(text, "<<<", "< < <")
	}
	for strings.Contains(text, ">>>") {
		text = strings.ReplaceAll(text, ">>>", "> > >")
	}
	return lookAlikeRe.ReplaceAllString(text, "SLOWHUNCH_INTAKE")
}

// Fence wraps the neutralized text between an opening and a closing
// delimiter line.
func Fence(text, delim string) string {
	return "<<<" + delim + ">>>\n" + Neutralize(text) + "\n<<<END " + delim + ">>>"
}
