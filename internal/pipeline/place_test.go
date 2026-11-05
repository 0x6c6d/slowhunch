package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

var plPrompt = provider.Prompt{Role: "place", Version: "place-1", Text: "place-sys"}

func plCfg() vault.PlaceConfig { return vault.DefaultConfig().Place }

func plItem(medium string) Item {
	return Item{Medium: medium, Title: "T", Plain: "x", Stored: []byte("x\n"), Delim: delim}
}

func runPlace(t *testing.T, a Atom, medium, reply string, nbs []Neighbour) (Decision, *fake.Fake) {
	t.Helper()
	f := &fake.Fake{Replies: map[string][]string{"place": {reply}}}
	d, _, err := Place(context.Background(), f, plPrompt, a, plItem(medium), nbs, plCfg())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(f.CallsFor("place")); n != 1 {
		t.Fatalf("calls = %d, want 1", n)
	}
	return d, f
}

func one(id string, c float64, tags ...string) []Neighbour {
	return []Neighbour{{ID: id, Cosine: c, Excerpt: "ex", Tags: tags}}
}

func TestPlaceDuplicateLowCosine(t *testing.T) {
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "asserts"}, "article", `{"verdict":"duplicate","target":"n1"}`, one("n1", 0.5))
	if d.Action != "extends" || d.Target != "n1" || d.Cosine != 0.5 {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceDuplicateHigh(t *testing.T) {
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "asserts"}, "article", `{"verdict":"duplicate","target":"n1"}`, one("n1", 0.9))
	if d.Action != "duplicate" || d.Target != "n1" {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceReads(t *testing.T) {
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "reads"}, "poem", `{"verdict":"duplicate","target":"n1"}`, one("n1", 0.95))
	if d.Action != "rereads" || d.Target != "n1" {
		t.Fatalf("%+v", d)
	}
	if d := Decide(Atom{Stance: "reads"}, true, Verdict{Verdict: "duplicate", Target: "n1"}, one("n1", 0.5), plCfg()); d.Action != "extends" {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceEcho(t *testing.T) {
	nbs := []Neighbour{{ID: "a", Cosine: 0.3}, {ID: "b", Cosine: 0.4}, {ID: "far", Cosine: 0.7, Far: true}}
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "crafts"}, "poem", `{"verdict":"new"}`, nbs)
	if d.Action != "new" || d.Target != "" || !d.Echo || d.EchoOf != "b" {
		t.Fatalf("%+v", d)
	}
	// not aesthetic: no echo
	if d := Decide(Atom{Stance: "asserts"}, false, Verdict{Verdict: "new"}, nbs, plCfg()); d.Echo {
		t.Fatalf("%+v", d)
	}
	// outside the band: no echo
	if d := Decide(Atom{Stance: "crafts"}, true, Verdict{Verdict: "new"}, one("x", 0.8), plCfg()); d.Echo {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceContradictNoTag(t *testing.T) {
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "asserts", Tags: []string{"rates"}}, "article", `{"verdict":"contradicts","target":"n1"}`, one("n1", 0.1, "other"))
	if d.Action != "extends" || d.Target != "n1" {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceContradictSharedTag(t *testing.T) {
	d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "asserts", Tags: []string{"rates"}}, "article", `{"verdict":"contradicts","target":"n1"}`, one("n1", 0.1, "rates"))
	if d.Action != "contradicts" || d.Target != "n1" {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceBadTarget(t *testing.T) {
	for _, target := range []string{"../../etc/passwd", "ghost", "", "N1"} {
		for _, v := range []string{"duplicate", "extends", "contradicts"} {
			reply := `{"verdict":"` + v + `","target":"` + target + `"}`
			d, _ := runPlace(t, Atom{Claim: "c", Evidence: "e", Stance: "asserts"}, "article", reply, one("n1", 0.99))
			if d.Action != "new" || d.Target != "" {
				t.Fatalf("%s %q: %+v", v, target, d)
			}
		}
	}
	// unknown verdict is rejected by validation
	f := &fake.Fake{Replies: map[string][]string{"place": {`{"verdict":"merge","target":"n1"}`, `{"verdict":"merge"}`, `{"verdict":"merge"}`, `{"verdict":"merge"}`}}}
	_, _, err := Place(context.Background(), f, plPrompt, Atom{Stance: "asserts"}, plItem("article"), one("n1", 0.99), plCfg())
	if errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("err = %v", err)
	}
	if d := Decide(Atom{Stance: "asserts"}, false, Verdict{Verdict: "merge", Target: "n1"}, one("n1", 0.99), plCfg()); d.Action != "new" || d.Target != "" {
		t.Fatalf("%+v", d)
	}
}

func TestPlaceExcerptCap(t *testing.T) {
	ev := strings.Repeat("é", 500)
	cands := []Candidate{{ID: "n1", Text: "interest rates rose sharply", Evidence: ev, Stance: "asserts"}}
	a := Atom{Claim: "interest rates rose", Evidence: "rates rose", Stance: "asserts"}
	nbs := BuildNeighbours(a, false, cands, plCfg())
	if len(nbs) != 1 || nbs[0].ID != "n1" || len([]rune(nbs[0].Excerpt)) != 300 {
		t.Fatalf("%+v", nbs)
	}
	_, f := runPlace(t, a, "article", `{"verdict":"extends","target":"n1"}`, nbs)
	u := f.CallsFor("place")[0].User
	if strings.Contains(u, strings.Repeat("é", 301)) || !strings.Contains(u, strings.Repeat("é", 300)) {
		t.Fatal("excerpt not capped at 300 in request")
	}
	if CapExcerpt("héllo", 2) != "hé" || CapExcerpt("ab", 5) != "ab" {
		t.Fatal("CapExcerpt")
	}
}

func TestPlaceOneCallPerAtom(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"place": {`{"verdict":"new"}`, `{"verdict":"extends","target":"n1"}`}}}
	atoms := []Atom{
		{Claim: "first claim", Evidence: "first evidence", Stance: "asserts"},
		{Claim: "second claim", Evidence: "second evidence", Stance: "asserts"},
	}
	nbs := []Neighbour{{ID: "n1", Cosine: 0.42, Excerpt: "old excerpt"}, {ID: "p2", Cosine: 0.1, Far: true, Excerpt: "far one"}}
	it := plItem("article")
	for i, a := range atoms {
		if _, _, err := Place(context.Background(), f, plPrompt, a, it, nbs, plCfg()); err != nil {
			t.Fatal(err)
		}
		calls := f.CallsFor("place")
		if len(calls) != i+1 {
			t.Fatalf("calls = %d after atom %d", len(calls), i)
		}
		c := calls[i]
		if c.System != "place-sys" || !strings.Contains(c.User, a.Claim) || !strings.Contains(c.User, a.Evidence) {
			t.Fatalf("request: %+v", c)
		}
		other := atoms[1-i]
		if strings.Contains(c.User, other.Claim) {
			t.Fatal("atoms batched")
		}
		for _, want := range []string{delim, "n1", "0.42", "p2", "far: true", "old excerpt", "far one"} {
			if !strings.Contains(c.User, want) {
				t.Fatalf("user missing %q:\n%s", want, c.User)
			}
		}
	}
}

func TestPlaceNoNeighbours(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{}}
	d, _, err := Place(context.Background(), f, plPrompt, Atom{Claim: "c", Evidence: "e", Stance: "asserts"}, plItem("article"), nil, plCfg())
	if err != nil || d.Action != "new" || d.Target != "" || len(f.Calls) != 0 {
		t.Fatalf("%+v %v calls=%d", d, err, len(f.Calls))
	}
	if nbs := BuildNeighbours(Atom{Claim: "c"}, false, nil, plCfg()); len(nbs) != 0 {
		t.Fatal(nbs)
	}
}

func TestPlaceBuildNeighboursFar(t *testing.T) {
	cands := []Candidate{
		{ID: "a1", Text: "interest rates rose", Evidence: "e1"},
		{ID: "a2", Text: "rates fell later", Evidence: "e2"},
		{ID: "p1", Text: "rates of the leaves rose", Evidence: "e3", Aesthetic: true},
	}
	cfg := plCfg()
	cfg.Near = 1
	nbs := BuildNeighbours(Atom{Claim: "interest rates rose", Evidence: "rates rose"}, false, cands, cfg)
	if len(nbs) != 2 || nbs[0].ID != "a1" || nbs[0].Far || nbs[1].ID != "p1" || !nbs[1].Far || !nbs[1].Aesthetic {
		t.Fatalf("%+v", nbs)
	}
}
