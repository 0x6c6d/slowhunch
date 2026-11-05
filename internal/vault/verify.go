package vault

import (
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Verify re-derives the index from the files and reports inconsistencies; it never fixes.
// It returns problems sorted by path, line, code and the number of files checked.
func Verify(v *Vault) ([]Problem, int, error) {
	ix, lerrs, err := v.LoadIndex()
	if err != nil {
		return nil, 0, err
	}
	ps := []Problem{}
	for _, le := range lerrs {
		id := strings.TrimSuffix(le.Path[strings.LastIndex(le.Path, "/")+1:], ".md")
		ps = append(ps, Problem{ID: id, Path: le.Path, Code: "BAD_FRONTMATTER", Message: le.Err.Error()})
	}
	all := ix.All()
	for _, e := range all {
		d := e.Doc
		add := func(key, code, msg string) {
			ps = append(ps, Problem{ID: e.ID, Path: e.Path, Line: frontmatter.LineOf(e.Raw, key), Code: code, Message: msg})
		}
		if err := Validate(d, e.ID); err != nil {
			add("", "BAD_FIELD", err.Error())
		}
		if stored := d.GetString("rev"); stored != frontmatter.Rev(d) {
			add("rev", "REV_MISMATCH", "stored rev "+stored+" does not match computed rev "+frontmatter.Rev(d))
		}
		for _, id := range Links(d) {
			if _, ok := ix.Get(id); !ok {
				add("", "DANGLING_LINK", "link to missing id "+id)
			}
		}
		if e.Type != "note" {
			continue
		}
		loc := strings.TrimSpace(d.GetString("locator"))
		var pl Locator
		locOK := false
		if loc != "" {
			if l, err := ParseLocator(loc); err != nil {
				add("locator", "LOCATOR_INVALID", err.Error())
			} else {
				pl, locOK = l, true
			}
		}
		srcID := d.GetString("source")
		src, ok := ix.Get(srcID)
		if srcID == "" || !ok || src.Type != "source" || src.Doc.GetString("text_hash") == "" {
			continue
		}
		medium := src.Doc.GetString("medium")
		stored, err := v.ReadFile("inbox/" + srcID + ".txt")
		if err != nil {
			// reported once, on the source
			dup := false
			for _, p := range ps {
				if p.Code == "INBOX_MISSING" && p.ID == srcID {
					dup = true
				}
			}
			if !dup {
				ps = append(ps, Problem{ID: srcID, Path: src.Path, Line: frontmatter.LineOf(src.Raw, "text_hash"), Code: "INBOX_MISSING", Message: "inbox/" + srcID + ".txt is missing"})
			}
			continue
		}
		ev := d.GetString("evidence")
		if !strings.Contains(InboxPlain(stored, medium), ev) {
			add("evidence", "EVIDENCE_MISMATCH", "evidence is not an exact substring of the stored source text")
		}
		if locOK {
			if from, to, ok := pl.LineRange(); ok {
				lines := InboxLines(stored, medium)
				if from < 1 || to > len(lines) || from > to {
					add("locator", "LOCATOR_RANGE", "locator is outside the source's line range")
				} else if !strings.Contains(strings.Join(lines[from-1:to], "\n"), ev) {
					add("locator", "LOCATOR_RANGE", "evidence is not within the located lines")
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
	return ps, len(all) + len(lerrs), nil
}
