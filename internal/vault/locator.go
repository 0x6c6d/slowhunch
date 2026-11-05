package vault

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Locator is a parsed position inside a source.
type Locator struct {
	Kind     string // line lines stanza verse comment section timecode region
	From, To int
	Name     string
}

var (
	nRe     = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)
	rangeRe = regexp.MustCompile(`^([1-9][0-9]{0,8})-([1-9][0-9]{0,8})$`)
	tcRe    = regexp.MustCompile(`^(\d{1,3}):([0-5]\d)-(\d{1,3}):([0-5]\d)$`)
	regRe   = regexp.MustCompile(`^(\d{1,9}),(\d{1,9}),(\d{1,9}),(\d{1,9})$`)
)

// ParseLocator parses the closed locator grammar.
func ParseLocator(s string) (Locator, error) {
	fail := func() (Locator, error) { return Locator{}, bad("invalid locator %q", s) }
	kind, arg, ok := strings.Cut(s, " ")
	if !ok || arg == "" {
		return fail()
	}
	switch kind {
	case "line", "stanza", "verse", "comment":
		if !nRe.MatchString(arg) {
			return fail()
		}
		n, _ := strconv.Atoi(arg)
		return Locator{Kind: kind, From: n, To: n}, nil
	case "lines":
		m := rangeRe.FindStringSubmatch(arg)
		if m == nil {
			return fail()
		}
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		if a > b {
			return fail()
		}
		return Locator{Kind: kind, From: a, To: b}, nil
	case "section":
		if len([]rune(arg)) > 80 || strings.ContainsAny(arg, "\n\r") || strings.TrimSpace(arg) == "" {
			return fail()
		}
		return Locator{Kind: kind, Name: arg}, nil
	case "timecode":
		m := tcRe.FindStringSubmatch(arg)
		if m == nil {
			return fail()
		}
		v := make([]int, 4)
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		from, to := v[0]*60+v[1], v[2]*60+v[3]
		if from >= to {
			return fail()
		}
		return Locator{Kind: kind, From: from, To: to}, nil
	case "region":
		m := regRe.FindStringSubmatch(arg)
		if m == nil {
			return fail()
		}
		v := make([]int, 4)
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		if v[0] >= v[2] || v[1] >= v[3] {
			return fail()
		}
		return Locator{Kind: kind, From: v[0], To: v[1], Name: fmt.Sprintf("%d,%d,%d,%d", v[0], v[1], v[2], v[3])}, nil
	}
	return fail()
}

// LineRange returns the line range for line/lines locators.
func (l Locator) LineRange() (from, to int, ok bool) {
	if l.Kind == "line" || l.Kind == "lines" {
		return l.From, l.To, true
	}
	return 0, 0, false
}
