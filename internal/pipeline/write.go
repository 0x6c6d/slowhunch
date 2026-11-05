package pipeline

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/search"
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

// modDoc is an existing vault file this run changes by adding data.
type modDoc struct {
	id, path, rev string
	orig          []byte
	doc           *frontmatter.Doc
}

// applied is the in-memory result of applying every decision of a run.
type applied struct {
	source *frontmatter.Doc
	newIDs []string // written notes, in plan order
	notes  map[string]*frontmatter.Doc
	mods   []*modDoc // changed existing files, sorted by id
}

func addUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// validateDoc checks d against the schema.
func validateDoc(d *frontmatter.Doc, id string) error {
	return vault.Validate(d, id)
}

// applyOutcomes turns the placed atoms into new documents, additions to
// existing files (corroborations, hunch evidence) and report entries.
func applyOutcomes(ix *vault.Index, plan []placed, it Item, writer, today, textHash string, cfg vault.PlaceConfig, rep *report.Report) (*applied, error) {
	ap := &applied{notes: map[string]*frontmatter.Doc{}}
	var written []placed
	for _, p := range plan {
		if p.Decision.Action != "duplicate" {
			written = append(written, p)
			ap.notes[p.ID] = noteDoc(p, it, writer, today)
			ap.newIDs = append(ap.newIDs, p.ID)
		}
	}
	ap.source = sourceDoc(it, writer, today, textHash, written)

	mods := map[string]*modDoc{}
	// peek returns the current in-memory form of id without marking it changed.
	peek := func(id string) *frontmatter.Doc {
		if d, ok := ap.notes[id]; ok {
			return d
		}
		if m, ok := mods[id]; ok {
			return m.doc
		}
		if e, ok := ix.Get(id); ok {
			return e.Doc
		}
		return nil
	}
	// edit returns a changeable form of id; existing files are cloned once.
	edit := func(id string) *frontmatter.Doc {
		if d, ok := ap.notes[id]; ok {
			return d
		}
		if m, ok := mods[id]; ok {
			return m.doc
		}
		e, ok := ix.Get(id)
		if !ok {
			return nil
		}
		m := &modDoc{id: id, path: e.Path, rev: frontmatter.Rev(e.Doc), orig: e.Doc.Bytes(), doc: e.Doc.Clone()}
		mods[id] = m
		return m.doc
	}
	isHunch := func(id string) bool {
		e, ok := ix.Get(id)
		return ok && e.Type == "hunch"
	}
	addEvidence := func(hid, nid, relation string) {
		d := edit(hid)
		ev := d.GetMaps("evidence")
		for _, m := range ev {
			if m["note"] == nid {
				return
			}
		}
		ev = append(append([]map[string]string{}, ev...), map[string]string{"date": today, "note": nid, "relation": relation})
		d.Set("evidence", ev)
		if relation == "supports" && d.GetString("state") == "open" {
			d.Set("state", "warming")
		}
		rep.HunchesMoved = append(rep.HunchesMoved, report.HunchMove{Hunch: report.Link(hid), Note: report.Link(nid), Relation: relation})
	}

	// Open and warming hunches are matched by TF-IDF over their questions.
	var hdocs []search.Doc
	for _, e := range ix.OfType("hunch") {
		if s := e.Doc.GetString("state"); s == "open" || s == "warming" {
			hdocs = append(hdocs, search.Doc{ID: e.ID, Text: e.Doc.GetString("question")})
		}
	}
	var hix *search.Index
	if len(hdocs) > 0 {
		hix = search.Build(hdocs)
	}
	matchHunches := func(a Atom) []string {
		var out []string
		if it.Hunch != "" && isHunch(it.Hunch) {
			out = append(out, it.Hunch)
		}
		if hix == nil {
			return out
		}
		q := hix.Query(atomText(a))
		for _, h := range hdocs {
			if search.Cosine(q, hix.DocVector(h.ID)) >= cfg.ContradictCosine {
				out = addUnique(out, h.ID)
			}
		}
		return out
	}

	sid := it.SourceID
	counts := map[string]int{"duplicates": 0, "contradictions": 0, "echoes": 0}
	for _, p := range plan {
		dec := p.Decision
		ref := report.AtomRef{ID: report.Link(p.ID), Claim: p.Atom.Claim, Stance: p.Atom.Stance, Locator: p.Atom.Locator, Action: dec.Action}
		if dec.Target != "" && dec.Action != "new" {
			ref.Target = report.Link(dec.Target)
		}
		switch dec.Action {
		case "duplicate":
			// The atom merges into its target: no new note.
			ref.ID = report.Link(dec.Target)
			d := edit(dec.Target)
			if d == nil {
				return nil, fmt.Errorf("duplicate target %s not found", dec.Target)
			}
			entry := today + " " + sid
			cur := d.GetList("corroborations")
			d.Set("corroborations", addUnique(append([]string{}, cur...), entry))
			rep.Corroborations = append(rep.Corroborations, report.Corroboration{Atom: report.Link(dec.Target), Source: report.Link(sid), Date: today})
			counts["duplicates"]++
		case "contradicts":
			nd := ap.notes[p.ID]
			nd.Set("status", "contradiction")
			var hs []string
			if t := peek(dec.Target); t != nil {
				for _, h := range t.GetList("bears_on") {
					if isHunch(h) {
						hs = addUnique(hs, h)
					}
				}
			}
			if it.Hunch != "" && isHunch(it.Hunch) {
				hs = addUnique(hs, it.Hunch)
			}
			links := []string{}
			for _, h := range hs {
				nd.Set("bears_on", addUnique(append([]string{}, nd.GetList("bears_on")...), h))
				addEvidence(h, p.ID, "weakens")
				links = append(links, report.Link(h))
			}
			rep.Contradictions = append(rep.Contradictions, report.Contradiction{Atom: report.Link(p.ID), Contradicts: report.Link(dec.Target), Hunches: links, Cosine: dec.Cosine})
			counts["contradictions"]++
		default: // new, extends, rereads
			nd := ap.notes[p.ID]
			for _, h := range matchHunches(p.Atom) {
				nd.Set("bears_on", addUnique(append([]string{}, nd.GetList("bears_on")...), h))
				addEvidence(h, p.ID, "supports")
			}
			if dec.Echo && dec.EchoOf != "" {
				rep.Echoes = append(rep.Echoes, report.Echo{Atom: report.Link(p.ID), Echoes: report.Link(dec.EchoOf), Cosine: dec.Cosine})
				counts["echoes"]++
			}
		}
		rep.Atoms = append(rep.Atoms, ref)
	}
	for k, n := range counts {
		rep.Counts[k] = n
	}

	if err := vault.Validate(ap.source, sid); err != nil {
		return nil, err
	}
	for _, id := range ap.newIDs {
		if err := validateDoc(ap.notes[id], id); err != nil {
			return nil, err
		}
	}
	for _, m := range mods {
		if string(m.doc.Bytes()) == string(m.orig) {
			continue
		}
		m.doc.Set("updated", today)
		if err := validateDoc(m.doc, m.id); err != nil {
			return nil, err
		}
		ap.mods = append(ap.mods, m)
	}
	sort.Slice(ap.mods, func(i, j int) bool { return ap.mods[i].id < ap.mods[j].id })
	return ap, nil
}
