package vault

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Vocabulary of the vault schema.
var (
	Types       = []string{"note", "source", "hunch", "proposal", "topic"}
	Mediums     = []string{"paper", "article", "blog", "post", "thread", "page", "code", "book", "own", "poem", "lyric", "prose", "quote", "film", "art", "talk", "other"}
	Stances     = []string{"asserts", "crafts", "reads"}
	HunchStates = []string{"open", "warming", "tested", "dropped"}
)

var (
	noteStatuses     = []string{"active", "contradiction"}
	proposalStatuses = []string{"open", "testing", "done", "dropped"}
	relations        = []string{"supports", "weakens"}
)

// Folder returns the folder for a type, or "" if unknown.
func Folder(typ string) string {
	switch typ {
	case "note":
		return "notes"
	case "source":
		return "sources"
	case "hunch":
		return "hunches"
	case "proposal":
		return "proposals"
	case "topic":
		return "topics"
	}
	return ""
}

func has(set []string, s string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// IsAesthetic reports whether the medium is an aesthetic one.
func IsAesthetic(m string) bool {
	return has([]string{"poem", "lyric", "prose", "quote", "film", "art"}, m)
}

// IsFactual reports whether the medium is a factual one.
func IsFactual(m string) bool {
	return has([]string{"paper", "article", "blog", "page", "post", "thread", "code", "talk", "book"}, m)
}

const commonFields = "id type created updated author rev tags bears_on links"

var typeFields = map[string]string{
	"note":     "claim evidence stance locator source status unlocks repurpose confidence rereads rendering translates corroborations",
	"hunch":    "question state next_test evidence revisit",
	"source":   "url medium title author published atoms translates owner_note text_hash added_by",
	"proposal": "items idea cheapest_test status",
	"topic":    "title",
}

var typeRequired = map[string]string{
	"note":     "claim evidence stance",
	"hunch":    "question state",
	"source":   "medium title",
	"proposal": "idea status",
	"topic":    "title",
}

var listFields = map[string]bool{"tags": true, "bears_on": true, "links": true, "unlocks": true, "rereads": true, "atoms": true, "items": true, "corroborations": true}

var idListFields = []string{"bears_on", "links", "unlocks", "rereads", "atoms", "items", "corroborations"}

var (
	idRe     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	authorRe = regexp.MustCompile(`^agent:[^/\s]+/\S*$`)
	dateRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	confRe   = regexp.MustCompile(`^(0(\.\d+)?|1(\.0+)?)$`)
)

func bad(format string, a ...any) error {
	return errs.New(errs.BadInput, fmt.Sprintf(format, a...), "")
}

func validDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// Validate checks a document against the schema; fileID is the id implied by the file name.
func Validate(d *frontmatter.Doc, fileID string) error {
	typ := d.GetString("type")
	if !has(Types, typ) {
		return bad("unknown type %q", typ)
	}
	writer := "author"
	if typ == "source" {
		writer = "added_by"
	}
	allowed := map[string]bool{}
	for _, f := range strings.Fields(commonFields + " " + typeFields[typ]) {
		allowed[f] = true
	}
	if typ == "source" {
		// added_by is the writer; author is the work's author (both allowed).
		allowed["added_by"] = true
	} else {
		delete(allowed, "added_by")
	}
	for _, f := range d.Fields {
		if !allowed[f.Key] {
			return bad("unknown field %q for type %s", f.Key, typ)
		}
		switch {
		case typ == "hunch" && f.Key == "evidence":
			if d.GetMaps("evidence") == nil {
				return bad("field %q must be a list of maps", f.Key)
			}
		case listFields[f.Key]:
			if d.GetList(f.Key) == nil {
				return bad("field %q must be a list", f.Key)
			}
		default:
			if _, ok := f.Value.(string); !ok {
				return bad("field %q must be a scalar", f.Key)
			}
		}
	}
	req := strings.Fields("id type created updated rev " + typeRequired[typ])
	req = append(req, writer)
	for _, k := range req {
		if _, ok := d.Get(k); !ok || (!listFields[k] && k != "evidence" && d.GetString(k) == "") {
			return bad("missing required field %q", k)
		}
	}
	if fileID != "" && d.GetString("id") != fileID {
		return bad("id %q does not match file id %q", d.GetString("id"), fileID)
	}
	if !idRe.MatchString(d.GetString("id")) {
		return bad("field %q is not a valid id", "id")
	}
	for _, k := range []string{"created", "updated", "published", "revisit"} {
		if v := d.GetString(k); v != "" && !validDate(v) {
			return bad("field %q: bad date %q", k, v)
		}
	}
	for _, k := range []string{"author", "added_by"} {
		if v, ok := d.Get(k); ok {
			s, _ := v.(string)
			if typ == "source" && k == "author" {
				continue // the work's author is free text
			}
			if s != "human" && !authorRe.MatchString(s) {
				return bad("field %q: bad value %q", k, s)
			}
		}
	}
	for _, k := range idListFields {
		for _, id := range d.GetList(k) {
			if !idRe.MatchString(id) {
				return bad("field %q: bad id %q", k, id)
			}
		}
	}
	enum := func(k string, set []string) error {
		if v, ok := d.Get(k); ok {
			if s, _ := v.(string); !has(set, s) {
				return bad("field %q: bad value %q", k, s)
			}
		}
		return nil
	}
	var err error
	switch typ {
	case "note":
		if err = enum("stance", Stances); err != nil {
			return err
		}
		if err = enum("status", noteStatuses); err != nil {
			return err
		}
		if strings.ContainsAny(d.GetString("claim"), "\n\r") {
			return bad("field %q must be one line", "claim")
		}
		if v, ok := d.Get("confidence"); ok {
			s, _ := v.(string)
			if !confRe.MatchString(s) {
				return bad("field %q: bad value %q", "confidence", s)
			}
			if d.GetString("stance") != "asserts" {
				return bad("field %q only allowed when stance is asserts", "confidence")
			}
		}
		if v, ok := d.Get("locator"); ok {
			s, _ := v.(string)
			if _, err := ParseLocator(s); err != nil {
				return bad("field %q: %v", "locator", err)
			}
		}
	case "hunch":
		if err = enum("state", HunchStates); err != nil {
			return err
		}
		for i, m := range d.GetMaps("evidence") {
			for k, v := range m {
				switch k {
				case "date":
					if !validDate(v) {
						return bad("evidence[%d].date: bad date %q", i, v)
					}
				case "note":
				case "relation":
					if !has(relations, v) {
						return bad("evidence[%d].relation: bad value %q", i, v)
					}
				default:
					return bad("evidence[%d]: unknown key %q", i, k)
				}
			}
		}
	case "source":
		if err = enum("medium", Mediums); err != nil {
			return err
		}
	case "proposal":
		if err = enum("status", proposalStatuses); err != nil {
			return err
		}
	}
	return nil
}
