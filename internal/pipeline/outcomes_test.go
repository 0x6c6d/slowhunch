package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// scriptFake keeps at the gate, returns atoms from extract and the given
// place verdict (JSON) for every place call.
func scriptFake(atoms []Atom, place string) *fake.Fake {
	return &fake.Fake{Handler: func(req provider.Request) (provider.Response, error) {
		switch req.Role {
		case "gate":
			return provider.Response{Text: `{"keep": true, "reason": "ok"}`}, nil
		case "extract":
			b, _ := json.Marshal(map[string]any{"atoms": atoms})
			return provider.Response{Text: string(b)}, nil
		case "place":
			if place == "" {
				place = `{"verdict": "new", "target": "", "reason": "fresh"}`
			}
			return provider.Response{Text: place}, nil
		}
		return provider.Response{}, nil
	}}
}

func verdict(v, target string) string {
	b, _ := json.Marshal(Verdict{Verdict: v, Target: target, Reason: "test"})
	return string(b)
}

func seedHunch(t *testing.T, v *vault.Vault, r *gitops.Repo, id, question, state string) {
	t.Helper()
	d := &frontmatter.Doc{Fields: []frontmatter.Field{
		field("id", id), field("type", "hunch"), field("created", "2026-11-01"), field("updated", "2026-11-01"),
		field("author", "human"), field("rev", "000000"), field("tags", []string{}), field("bears_on", []string{}),
		field("question", question), field("state", state), field("evidence", []map[string]string{}),
	}}
	if err := vault.Validate(d, id); err != nil {
		t.Fatal(err)
	}
	if _, err := v.PutDoc("hunches/"+id+".md", d, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit([]string{"hunches/" + id + ".md"}, "seed hunch", ""); err != nil {
		t.Fatal(err)
	}
}

func mustAdd(t *testing.T, v *vault.Vault, r *gitops.Repo, f *fake.Fake, o Options) *report.Report {
	t.Helper()
	rep, err := Add(context.Background(), addDeps(v, r, f), o)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func unlink(s string) string { return strings.TrimSuffix(strings.TrimPrefix(s, "[["), "]]") }

func loadDoc(t *testing.T, v *vault.Vault, rel string) *frontmatter.Doc {
	t.Helper()
	b, err := v.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	d, err := frontmatter.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func noteFiles(t *testing.T, v *vault.Vault) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(v.Root, "notes"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func committedFiles(t *testing.T, dir string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(out))
	sort.Strings(f)
	return f
}

func commitsSince(t *testing.T, dir, base string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-list", "--count", base+"..HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// cosineBetween is the TF-IDF cosine place sees between a seeded atom (the only candidate) and a new atom.
func cosineBetween(seed, a Atom, aesthetic bool) float64 {
	nbs := BuildNeighbours(a, aesthetic, []Candidate{{
		ID: "seed", Text: seed.Claim + "\n" + seed.Evidence + "\n" + strings.Join(seed.Tags, " "),
		Evidence: seed.Evidence, Stance: seed.Stance, Tags: seed.Tags, Aesthetic: aesthetic,
	}}, vault.DefaultConfig().Place)
	if len(nbs) == 0 {
		return 0
	}
	return nbs[0].Cosine
}

func TestAddLarkin(t *testing.T) {
	v, r := addVault(t)
	seedHunch(t, v, r, "h1", "Does deferral in a line read as meaning?", "open")
	head := headOf(t, v.Root)
	poem := "The trees are coming into leaf\nLike something almost being said;"
	atoms := []Atom{
		{Claim: "The line break holds the sentence open like a bud", Evidence: poem, Stance: "crafts", Locator: "lines 1-2", Tags: []string{"lineation"}},
		{Claim: "Spring reads as speech not quite spoken", Evidence: "Like something almost being said;", Stance: "reads", Locator: "line 2", Tags: []string{"spring"}},
	}
	rep := mustAdd(t, v, r, scriptFake(atoms, ""), Options{Arg: poem, Medium: "poem", Title: "Trees", Author: "Philip Larkin", Hunch: "h1"})

	sid := unlink(rep.Source)
	if !regexp.MustCompile(`^poem-larkin-trees-[0-9a-f]{4}$`).MatchString(sid) {
		t.Fatalf("source id %q", sid)
	}
	crafts, reads := "crafts-larkin-trees-lines-1-2", "reads-larkin-trees-line-2"
	for _, id := range []string{crafts, reads} {
		if !v.Exists("notes/" + id + ".md") {
			t.Fatalf("missing note %s (atoms %+v)", id, rep.Atoms)
		}
		if bo := loadDoc(t, v, "notes/"+id+".md").GetList("bears_on"); len(bo) != 1 || bo[0] != "h1" {
			t.Fatalf("%s bears_on %v", id, bo)
		}
	}
	inbox, err := v.ReadFile("inbox/" + sid + ".txt")
	if err != nil || string(inbox) != "1\tThe trees are coming into leaf\n2\tLike something almost being said;\n" {
		t.Fatalf("inbox %q %v", inbox, err)
	}
	h := loadDoc(t, v, "hunches/h1.md")
	ev := h.GetMaps("evidence")
	if len(ev) != 2 || h.GetString("state") != "warming" {
		t.Fatalf("h1 evidence %v state %s", ev, h.GetString("state"))
	}
	seen := map[string]bool{}
	for _, m := range ev {
		if m["relation"] != "supports" || m["date"] != "2026-11-05" {
			t.Fatalf("evidence entry %v", m)
		}
		seen[m["note"]] = true
	}
	if !seen[crafts] || !seen[reads] {
		t.Fatalf("evidence notes %v", ev)
	}
	if err := vault.Validate(h, "h1"); err != nil {
		t.Fatal(err)
	}
	if len(rep.HunchesMoved) != 2 {
		t.Fatalf("hunches moved %+v", rep.HunchesMoved)
	}
	if n := commitsSince(t, v.Root, head); n != "1" {
		t.Fatalf("%s commits", n)
	}
	want := []string{"hunches/h1.md", "inbox/" + sid + ".txt", "notes/" + crafts + ".md", "notes/" + reads + ".md", "sources/" + sid + ".md"}
	if got := committedFiles(t, v.Root); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("committed %v, want %v", got, want)
	}
}

func TestAddPlaceThresholds(t *testing.T) {
	type scenario struct {
		name           string
		medium         string
		seedText       string
		seed           Atom
		newText        string
		atom           Atom
		verdict        string
		lo, hi         float64
		wantAction     string
		check          func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, notesBefore int)
		seedT, seedAut string
	}
	seedArticle := "Interest rates rose by two points in 2024 according to the central bank report."
	seedAtom := Atom{Claim: "Interest rates rose two points in 2024", Evidence: "Interest rates rose by two points in 2024", Stance: "asserts", Tags: []string{"rates"}}
	cases := []scenario{
		{
			name: "duplicate at 0.5 extends", medium: "article", seedText: seedArticle, seed: seedAtom,
			newText: "Interest rates rose by two points in 2024 while housing prices stayed flat across the region.",
			atom:    Atom{Claim: "Housing stayed flat as rates rose", Evidence: "Interest rates rose by two points in 2024 while housing prices stayed flat", Stance: "asserts"},
			verdict: "duplicate", lo: 0.3, hi: 0.7, wantAction: "extends",
			check: func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, before int) {
				d := loadDoc(t, v, "notes/"+unlink(rep.Atoms[0].ID)+".md")
				if l := d.GetList("links"); len(l) != 1 || l[0] != seedID {
					t.Fatalf("links %v", l)
				}
				if len(rep.Corroborations) != 0 || len(noteFiles(t, v)) != before+1 {
					t.Fatalf("corroborations %v", rep.Corroborations)
				}
			},
		},
		{
			name: "duplicate at 0.9 corroborates", medium: "article", seedText: seedArticle, seed: seedAtom,
			newText: "Interest rates rose by two points in 2024, markets shrugged and bonds held.",
			atom:    seedAtom, verdict: "duplicate", lo: 0.9, hi: 1.01, wantAction: "duplicate",
			check: func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, before int) {
				if n := len(noteFiles(t, v)); n != before {
					t.Fatalf("a duplicate wrote a note file: %d -> %d", before, n)
				}
				sid := unlink(rep.Source)
				d := loadDoc(t, v, "notes/"+seedID+".md")
				if c := d.GetList("corroborations"); len(c) != 1 || c[0] != "2026-11-05 "+sid {
					t.Fatalf("corroborations %v", c)
				}
				if d.GetString("evidence") != seedAtom.Evidence || d.GetString("updated") != "2026-11-05" {
					t.Fatalf("target changed beyond adding data: %v", d.Fields)
				}
				if len(rep.Corroborations) != 1 || rep.Corroborations[0] != (report.Corroboration{Atom: report.Link(seedID), Source: report.Link(sid), Date: "2026-11-05"}) {
					t.Fatalf("report corroborations %+v", rep.Corroborations)
				}
				if rep.Counts["duplicates"] != 1 || !v.Exists("sources/"+sid+".md") {
					t.Fatalf("counts %v", rep.Counts)
				}
			},
		},
		{
			name: "reads duplicate at 0.95 rereads", medium: "article", seedText: seedArticle, seed: seedAtom,
			newText: "Interest rates rose by two points in 2024, which felt like a warning.",
			atom:    Atom{Claim: seedAtom.Claim, Evidence: seedAtom.Evidence, Stance: "reads", Tags: []string{"rates"}},
			verdict: "duplicate", lo: 0.95, hi: 1.01, wantAction: "rereads",
			check: func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, before int) {
				d := loadDoc(t, v, "notes/"+unlink(rep.Atoms[0].ID)+".md")
				if rr := d.GetList("rereads"); len(rr) != 1 || rr[0] != seedID {
					t.Fatalf("rereads %v", rr)
				}
				if len(rep.Corroborations) != 0 || len(noteFiles(t, v)) != before+1 {
					t.Fatal("reads atom was treated as a duplicate")
				}
			},
		},
		{
			name: "aesthetic new at 0.4 echoes", medium: "poem", seedT: "Dover", seedAut: "Arnold",
			seedText: "The sea is calm tonight\nThe tide is full, the moon lies fair",
			seed:     Atom{Claim: "Calm sea under a full moon", Evidence: "The sea is calm tonight", Stance: "crafts", Locator: "line 1"},
			newText:  "The sea is grey tonight\nGulls cry over the harbour wall",
			atom:     Atom{Claim: "Grey sea under no moon", Evidence: "The sea is grey tonight", Stance: "crafts", Locator: "line 1"},
			verdict:  "new", lo: 0.2, hi: 0.8, wantAction: "new",
			check: func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, before int) {
				if len(rep.Echoes) != 1 || rep.Echoes[0].Echoes != report.Link(seedID) || rep.Echoes[0].Atom != rep.Atoms[0].ID || rep.Counts["echoes"] != 1 {
					t.Fatalf("echoes %+v counts %v", rep.Echoes, rep.Counts)
				}
				if c := rep.Echoes[0].Cosine; c < 0.2 || c >= 0.8 {
					t.Fatalf("echo cosine %v", c)
				}
			},
		},
		{
			name: "contradicts without shared tag at 0.1 extends", medium: "article", seedText: seedArticle, seed: seedAtom,
			newText: "Bank lending to farmers doubled during the drought, and grain silos filled early.",
			atom:    Atom{Claim: "Lending to farmers doubled in the 2024 drought", Evidence: "Bank lending to farmers doubled during the drought", Stance: "asserts", Tags: []string{"farming"}},
			verdict: "contradicts", lo: 0.01, hi: 0.2, wantAction: "extends",
			check: func(t *testing.T, v *vault.Vault, rep *report.Report, seedID string, before int) {
				if len(rep.Contradictions) != 0 {
					t.Fatalf("contradictions %+v", rep.Contradictions)
				}
				if s := loadDoc(t, v, "notes/"+unlink(rep.Atoms[0].ID)+".md").GetString("status"); s != "active" {
					t.Fatalf("status %s", s)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			aesthetic := vault.IsAesthetic(c.medium)
			cos := cosineBetween(c.seed, c.atom, aesthetic)
			if cos < c.lo || cos >= c.hi {
				t.Fatalf("test texts give cosine %.3f, want [%v, %v)", cos, c.lo, c.hi)
			}
			v, r := addVault(t)
			title, author := c.seedT, c.seedAut
			if title == "" {
				title, author = "Rates", "Bank"
			}
			seedRep := mustAdd(t, v, r, scriptFake([]Atom{c.seed}, ""), Options{Arg: c.seedText, Medium: c.medium, Title: title, Author: author})
			if len(seedRep.Atoms) != 1 {
				t.Fatalf("seed %+v", seedRep)
			}
			seedID := unlink(seedRep.Atoms[0].ID)
			before := len(noteFiles(t, v))
			head := headOf(t, v.Root)
			rep := mustAdd(t, v, r, scriptFake([]Atom{c.atom}, verdict(c.verdict, seedID)), Options{Arg: c.newText, Medium: c.medium, Title: "Second", Author: "Other"})
			if len(rep.Atoms) != 1 || rep.Atoms[0].Action != c.wantAction {
				t.Fatalf("atoms %+v errors %+v", rep.Atoms, rep.Errors)
			}
			if c.wantAction != "new" && rep.Atoms[0].Target != report.Link(seedID) {
				t.Fatalf("target %q", rep.Atoms[0].Target)
			}
			c.check(t, v, rep, seedID, before)
			if n := commitsSince(t, v.Root, head); n != "1" {
				t.Fatalf("%s commits", n)
			}
			for _, id := range rep.IDs() {
				if _, ok := mustIndex(t, v).Get(id); !ok {
					t.Fatalf("report id %s not in vault", id)
				}
			}
		})
	}
}

func mustIndex(t *testing.T, v *vault.Vault) *vault.Index {
	t.Helper()
	ix, lerrs, err := v.LoadIndex()
	if err != nil || len(lerrs) != 0 {
		t.Fatal(err, lerrs)
	}
	return ix
}

func TestAddContradiction(t *testing.T) {
	v, r := addVault(t)
	seedHunch(t, v, r, "rates-rise", "Do central bank rate rises cool housing prices?", "open")
	seedText := "Housing prices fell after the central bank raised interest rates in 2024."
	seed := Atom{Claim: "Rate rises cooled housing prices", Evidence: "Housing prices fell after the central bank raised interest rates", Stance: "asserts", Tags: []string{"housing", "rates"}}
	seedRep := mustAdd(t, v, r, scriptFake([]Atom{seed}, ""), Options{Arg: seedText, Medium: "article", Title: "Rates", Author: "Bank", Hunch: "rates-rise"})
	target := unlink(seedRep.Atoms[0].ID)
	targetBytes, err := v.ReadFile("notes/" + target + ".md")
	if err != nil {
		t.Fatal(err)
	}
	head := headOf(t, v.Root)

	newText := "Housing prices kept climbing in 2024 even though interest rates were raised twice."
	atom := Atom{Claim: "Housing prices rose despite rate rises", Evidence: "Housing prices kept climbing in 2024 even though interest rates were raised twice", Stance: "asserts", Tags: []string{"housing"}}
	rep := mustAdd(t, v, r, scriptFake([]Atom{atom}, verdict("contradicts", target)), Options{Arg: newText, Medium: "article", Title: "Prices", Author: "Agent"})

	if len(rep.Atoms) != 1 || rep.Atoms[0].Action != "contradicts" {
		t.Fatalf("atoms %+v", rep.Atoms)
	}
	newID := unlink(rep.Atoms[0].ID)
	nd := loadDoc(t, v, "notes/"+newID+".md")
	if nd.GetString("status") != "contradiction" || strings.Join(nd.GetList("links"), ",") != target || nd.GetString("source") != unlink(rep.Source) {
		t.Fatalf("contradiction note %v", nd.Fields)
	}
	if err := vault.Validate(nd, newID); err != nil {
		t.Fatal(err)
	}
	after, err := v.ReadFile("notes/" + target + ".md")
	if err != nil || string(after) != string(targetBytes) {
		t.Fatalf("contradiction edited the target:\n%s", after)
	}
	h := loadDoc(t, v, "hunches/rates-rise.md")
	var weakens []map[string]string
	for _, m := range h.GetMaps("evidence") {
		if m["relation"] == "weakens" {
			weakens = append(weakens, m)
		}
	}
	if len(weakens) != 1 || weakens[0]["note"] != newID || weakens[0]["date"] != "2026-11-05" {
		t.Fatalf("hunch evidence %v", h.GetMaps("evidence"))
	}
	if len(rep.Contradictions) != 1 {
		t.Fatalf("contradictions %+v", rep.Contradictions)
	}
	c := rep.Contradictions[0]
	if c.Atom != report.Link(newID) || c.Contradicts != report.Link(target) || len(c.Hunches) != 1 || c.Hunches[0] != report.Link("rates-rise") || c.Cosine <= 0 {
		t.Fatalf("contradiction %+v", c)
	}
	if rep.Counts["contradictions"] != 1 {
		t.Fatalf("counts %v", rep.Counts)
	}

	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"contradictions", "hunches_moved", "atoms", "corroborations", "echoes", "wildcards", "collisions", "counts", "errors", "cost", "git"}
	last := -1
	for _, k := range keys {
		i := strings.Index(string(b), `"`+k+`":`)
		if i <= last {
			t.Fatalf("key %s out of order in %s", k, b)
		}
		last = i
	}
	saved, err := v.ReadFile(".slowhunch/reports/" + rep.RunID + ".json")
	if err != nil || !strings.Contains(string(saved), `"contradictions"`) {
		t.Fatalf("saved report %v", err)
	}
	if strings.Index(string(saved), `"contradictions"`) > strings.Index(string(saved), `"atoms"`) {
		t.Fatal("contradictions are not listed first")
	}
	ix := mustIndex(t, v)
	ids := rep.IDs()
	if len(ids) < 4 {
		t.Fatalf("ids %v", ids)
	}
	for _, id := range ids {
		if _, ok := ix.Get(id); !ok {
			t.Fatalf("report names %s but it is not on disk", id)
		}
	}
	if n := commitsSince(t, v.Root, head); n != "1" {
		t.Fatalf("%s commits", n)
	}
	if got := committedFiles(t, v.Root); !contains(got, "hunches/rates-rise.md") || contains(got, "notes/"+target+".md") {
		t.Fatalf("committed %v", got)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestAddGateError(t *testing.T) {
	v, r := addVault(t)
	before, head := treeHash(t, v.Root), headOf(t, v.Root)
	f := &fake.Fake{Handler: func(provider.Request) (provider.Response, error) { return provider.Response{}, errors.New("down") }}
	rep := mustAdd(t, v, r, f, Options{Arg: "Interest rates rose by two points in 2024 according to the central bank report.", Medium: "article", Title: "Rates"})
	if len(rep.Errors) != 1 || rep.Errors[0].Code != "PROVIDER_ERROR" || len(rep.Atoms) != 0 || rep.Counts["rejected"] != 1 || rep.Source != "" {
		t.Fatalf("report %+v", rep)
	}
	if len(f.CallsFor("extract")) != 0 {
		t.Fatal("extract called after a gate error")
	}
	if treeHash(t, v.Root) != before || headOf(t, v.Root) != head {
		t.Fatal("a gate error changed the vault")
	}
}

func TestAddForce(t *testing.T) {
	v, r := addVault(t)
	reject := func() *fake.Fake {
		f := addFake()
		inner := f.Handler
		f.Handler = func(req provider.Request) (provider.Response, error) {
			if req.Role == "gate" {
				return provider.Response{Text: `{"keep": false, "reason": "boring"}`}, nil
			}
			return inner(req)
		}
		return f
	}
	before, head := treeHash(t, v.Root), headOf(t, v.Root)
	rep := mustAdd(t, v, r, reject(), Options{Arg: addArticle, Medium: "article", Title: "Flood"})
	if rep.Counts["rejected"] != 1 || len(rep.Errors) != 1 || rep.Errors[0].Code != "REJECTED" || rep.Errors[0].Reason != "boring" {
		t.Fatalf("report %+v", rep)
	}
	if treeHash(t, v.Root) != before || headOf(t, v.Root) != head {
		t.Fatal("rejected item changed the vault")
	}
	f := reject()
	rep = mustAdd(t, v, r, f, Options{Arg: addArticle, Medium: "article", Title: "Flood", Force: true})
	if len(f.CallsFor("gate")) != 0 || rep.Counts["rejected"] != 0 || len(rep.Atoms) != 1 || rep.Git.Commit == "" {
		t.Fatalf("forced report %+v", rep)
	}
	if !v.Exists("notes/" + unlink(rep.Atoms[0].ID) + ".md") {
		t.Fatal("forced atom not written")
	}
}

func TestAddEvidenceDrops(t *testing.T) {
	v, r := addVault(t)
	rep := mustAdd(t, v, r, addFake(), Options{Arg: addArticle, Medium: "article", Title: "Flood"})
	var found bool
	for _, e := range rep.Errors {
		if e.Code == "EVIDENCE_NOT_FOUND" && e.Item == "atom 1" {
			found = true
		}
	}
	if !found || rep.Counts["dropped"] != 1 {
		t.Fatalf("errors %+v counts %v", rep.Errors, rep.Counts)
	}
	ix := mustIndex(t, v)
	notes := ix.OfType("note")
	if len(notes) != 1 {
		t.Fatalf("%d notes", len(notes))
	}
	for _, n := range notes {
		if n.Doc.GetString("claim") == "bad atom" || strings.Contains(n.Doc.GetString("evidence"), "not in the text") {
			t.Fatal("dropped atom was written")
		}
	}
	src, _ := ix.Get(unlink(rep.Source))
	if a := src.Doc.GetList("atoms"); len(a) != 1 {
		t.Fatalf("source atoms %v", a)
	}
}
