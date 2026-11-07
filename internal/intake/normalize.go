package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var wikilinkRe = regexp.MustCompile(`!?\[\[([^\[\]\n]+)\]\]`)

var normReplacer = strings.NewReplacer(
	" ", " ", " ", " ", " ", " ",
	"‘", "'", "’", "'", "‛", "'",
	"“", "\"", "”", "\"", "‟", "\"",
	"—", "--", "–", "-", "…", "...",
	"\ufeff", "", "\u200b", "",
)

// Normalize canonicalizes intake text for the given medium.
func Normalize(text, medium string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if medium == "code" {
		return strings.TrimRight(text, "\n")
	}
	text = normReplacer.Replace(text)
	for {
		next := wikilinkRe.ReplaceAllString(text, "$1")
		if next == text {
			break
		}
		text = next
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.ReplaceAll(l, "\t", " ")
		l = strings.Join(strings.Fields(l), " ")
		if l == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, l)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// TextHash returns the lowercase hex sha256 of the normalized text.
func TextHash(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
