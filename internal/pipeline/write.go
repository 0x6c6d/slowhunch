package pipeline

import (
	"sort"
	"strconv"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// placed is one kept atom with its id and placement decision.
type placed struct {
	ID       string
	Atom     Atom
	Decision Decision
}

func inboxPath(sid string) string  { return "inbox/" + sid + ".txt" }
func sourcePath(sid string) string { return "sources/" + sid + ".md" }
func notePath(id string) string    { return "notes/" + id + ".md" }

func field(k string, v any) frontmatter.Field { return frontmatter.Field{Key: k, Value: v} }

func bearsOn(hunch string) []string {
	if hunch == "" {
		return []string{}
	}
	return []string{hunch}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// sourceDoc assembles the source document for an add run.
func sourceDoc(it Item, writer, today, textHash string, atoms []placed) *frontmatter.Doc {
	author := it.Author
	if author == "" {
		author = "anonymous"
	}
	title := it.Title
	if title == "" {
		title = "untitled"
	}
	tagSet := map[string]bool{}
	ids := []string{}
	for _, p := range atoms {
		ids = append(ids, p.ID)
		for _, t := range p.Atom.Tags {
			tagSet[t] = true
		}
	}
	tags := []string{}
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	d := &frontmatter.Doc{}
	d.Fields = []frontmatter.Field{
		field("id", it.SourceID), field("type", "source"),
		field("created", today), field("updated", today),
		field("added_by", writer), field("rev", "000000"),
		field("tags", tags), field("bears_on", bearsOn(it.Hunch)),
	}
	if it.URL != "" {
		d.Fields = append(d.Fields, field("url", it.URL))
	}
	d.Fields = append(d.Fields, field("medium", it.Medium), field("title", title), field("author", author))
	if it.Note != "" {
		d.Fields = append(d.Fields, field("owner_note", it.Note))
	}
	d.Fields = append(d.Fields, field("text_hash", textHash), field("atoms", ids))
	return d
}

// noteDoc assembles the note document for one placed atom.
func noteDoc(p placed, it Item, writer, today string) *frontmatter.Doc {
	a := p.Atom
	d := &frontmatter.Doc{}
	d.Fields = []frontmatter.Field{
		field("id", p.ID), field("type", "note"),
		field("created", today), field("updated", today),
		field("author", writer), field("rev", "000000"),
		field("tags", nonNil(append([]string(nil), a.Tags...))), field("bears_on", bearsOn(it.Hunch)),
		field("claim", a.Claim), field("evidence", a.Evidence), field("stance", a.Stance),
	}
	if a.Locator != "" {
		d.Fields = append(d.Fields, field("locator", a.Locator))
	}
	d.Fields = append(d.Fields, field("source", it.SourceID), field("status", "active"))
	if a.Confidence != nil {
		d.Fields = append(d.Fields, field("confidence", strconv.FormatFloat(*a.Confidence, 'f', -1, 64)))
	}
	if a.Rendering != "" {
		d.Fields = append(d.Fields, field("rendering", a.Rendering))
	}
	switch p.Decision.Action {
	case "extends", "duplicate", "contradicts":
		d.Fields = append(d.Fields, field("links", []string{p.Decision.Target}))
	case "rereads":
		d.Fields = append(d.Fields, field("rereads", []string{p.Decision.Target}))
	}
	d.Body = vault.NoteBody(a.Claim, a.Evidence)
	return d
}
