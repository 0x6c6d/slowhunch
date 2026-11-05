package vault

import (
	"slices"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Problem is one lint or verify finding.
type Problem struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Lint checks the index against the vault rules; sorted by path, line, code.
func Lint(ix *Index, loadErrs []LoadError) []Problem {
	ps := []Problem{}
	for _, le := range loadErrs {
		id := strings.TrimSuffix(le.Path[strings.LastIndex(le.Path, "/")+1:], ".md")
		ps = append(ps, Problem{ID: id, Path: le.Path, Code: "BAD_FRONTMATTER", Message: le.Err.Error()})
	}
	notes := ix.OfType("note")
	for _, e := range notes {
		d := e.Doc
		add := func(key, code, msg string) {
			ps = append(ps, Problem{ID: e.ID, Path: e.Path, Line: frontmatter.LineOf(e.Raw, key), Code: code, Message: msg})
		}
		if IsAesthetic(ix.MediumOf(e.ID)) && strings.TrimSpace(d.GetString("locator")) == "" {
			add("locator", "MISSING_LOCATOR", "note on an aesthetic source needs a locator")
		}
		if d.GetString("confidence") != "" && d.GetString("stance") != "asserts" {
			add("confidence", "CONFIDENCE_NOT_ASSERTS", "confidence is only allowed on notes with stance asserts")
		}
		if r := d.GetString("rendering"); r != "" && (strings.TrimSpace(d.GetString("evidence")) == "" || r == d.GetString("evidence")) {
			add("rendering", "RENDERING_AS_EVIDENCE", "rendering must be a translation distinct from a non-empty evidence quote")
		}
		if d.GetString("stance") == "reads" {
			ev := strings.TrimSpace(d.GetString("evidence"))
			rereads := d.GetList("rereads")
			for _, a := range notes {
				if a.ID == e.ID || strings.TrimSpace(a.Doc.GetString("evidence")) != ev {
					continue
				}
				ac, rc := a.Doc.GetString("created"), d.GetString("created")
				if !(ac < rc || (ac == rc && a.ID < e.ID)) {
					continue
				}
				if !slices.Contains(rereads, a.ID) {
					add("rereads", "READS_WITHOUT_REREADS", "reads note repeats the evidence of "+a.ID+" but does not list it in rereads")
				}
			}
		}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Code < b.Code
	})
	return ps
}
