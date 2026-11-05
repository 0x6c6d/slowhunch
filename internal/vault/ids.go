package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// accentFold maps common lowercase Latin accented letters to ASCII.
var accentFold = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a",
	'æ': "ae", 'ç': "c", 'č': "c", 'ć': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ę': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i",
	'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u",
	'ý': "y", 'ÿ': "y", 'ß': "ss", 'š': "s", 'ś': "s", 'ž': "z", 'ź': "z", 'ż': "z",
	'ł': "l", 'ř': "r", 'ď': "d", 'ť': "t", 'ð': "d", 'þ': "th",
}

// Slug lowercases s, folds common accents, collapses every run of characters
// outside [a-z0-9] into a single '-', trims '-' at both ends and cuts the
// result to at most max characters (max <= 0 means no limit). The result only
// contains [a-z0-9-] and may be empty.
func Slug(s string, max int) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		var rep string
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			rep = string(r)
		default:
			rep = accentFold[r]
		}
		if rep == "" {
			dash = true
			continue
		}
		if dash && b.Len() > 0 {
			b.WriteByte('-')
		}
		dash = false
		b.WriteString(rep)
	}
	out := b.String()
	if max > 0 && len(out) > max {
		out = strings.TrimRight(out[:max], "-")
	}
	return out
}

// ShortHash returns the first 4 hex characters of sha256(normalizedText).
func ShortHash(normalizedText string) string {
	sum := sha256.Sum256([]byte(normalizedText))
	return hex.EncodeToString(sum[:2])
}

// SourceID builds <medium>-<author>-<title>-<short>, or own-<title>-<short>
// for own sources.
func SourceID(medium, author, title, normalizedText string) string {
	m := Slug(medium, 24)
	t := Slug(title, 24)
	if t == "" {
		t = "untitled"
	}
	short := ShortHash(normalizedText)
	if m == "own" {
		return "own-" + t + "-" + short
	}
	a := ""
	if words := strings.Fields(author); len(words) > 0 {
		a = Slug(words[len(words)-1], 24)
	}
	if a == "" {
		a = "anonymous"
	}
	parts := []string{m, a, t, short}
	if m == "" {
		parts = parts[1:]
	}
	return strings.Join(parts, "-")
}

// AtomID builds <stance>-<base>-<loc>, where base is the source id without its
// leading medium and trailing short hash.
func AtomID(stance, sourceID, locator, claim string) string {
	loc := Slug(locator, 24)
	if loc == "" {
		loc = Slug(claim, 24)
	}
	if loc == "" {
		loc = "untitled"
	}
	base := ""
	if sid := Slug(sourceID, 0); sid != "" {
		parts := strings.Split(sid, "-")
		if len(parts) > 2 {
			base = strings.Join(parts[1:len(parts)-1], "-")
		}
	}
	parts := make([]string, 0, 3)
	for _, p := range []string{Slug(stance, 0), base, loc} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "-")
}

// KebabID returns Slug(text, 60), or "untitled" when that is empty. Used for
// hunch and proposal ids.
func KebabID(text string) string {
	if s := Slug(text, 60); s != "" {
		return s
	}
	return "untitled"
}

// UniqueID returns id if it does not exist, else the first of id-2, id-3, ...
// that does not exist.
func UniqueID(id string, exists func(string) bool) string {
	if !exists(id) {
		return id
	}
	for n := 2; ; n++ {
		c := id + "-" + strconv.Itoa(n)
		if !exists(c) {
			return c
		}
	}
}
