package cli

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func TestPerfVerifyAndSearch(t *testing.T) {
	d := newVault(t)
	v, err := vault.Open(d)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(41))
	words := make([]string, 400)
	for i := range words {
		var sb strings.Builder
		for j := 0; j < 3+rng.Intn(5); j++ {
			sb.WriteByte(byte('a' + rng.Intn(26)))
		}
		words[i] = sb.String() + fmt.Sprint(i%7)
	}
	words = append(words, "alpha", "river", "silence")
	put := func(rel, text string) {
		doc, err := frontmatter.Parse([]byte(text))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if _, err := v.PutDoc(rel, doc, ""); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	mediums := []string{"article", "paper", "poem", "lyric", "quote", "own"}
	const nSrc, perSrc = 200, 10
	var noteIDs []string
	for s := 0; s < nSrc; s++ {
		sid := fmt.Sprintf("src-%03d", s)
		medium := mediums[s%len(mediums)]
		nLines := 10 + rng.Intn(11)
		lines := make([][]string, nLines)
		strs := make([]string, nLines)
		for i := range lines {
			n := 6 + rng.Intn(6)
			for j := 0; j < n; j++ {
				lines[i] = append(lines[i], words[rng.Intn(len(words))])
			}
			strs[i] = strings.Join(lines[i], " ")
		}
		if err := v.WriteFile("inbox/"+sid+".txt", vault.FormatInbox(strings.Join(strs, "\n"), medium)); err != nil {
			t.Fatal(err)
		}
		put("sources/"+sid+".md", fmt.Sprintf("---\nid: %s\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: %s\ntitle: Source %d\ntext_hash: hash%d\n---\n", sid, medium, s, s))
		for n := 0; n < perSrc; n++ {
			id := fmt.Sprintf("n-%03d-%02d", s, n)
			li := rng.Intn(nLines)
			w := lines[li]
			a := rng.Intn(len(w) - 2)
			ev := strings.Join(w[a:a+3], " ")
			extra := ""
			if vault.IsAesthetic(medium) {
				extra += fmt.Sprintf("locator: line %d\n", li+1)
			}
			extra += fmt.Sprintf("tags:\n  - t%d\n  - t%d\n", rng.Intn(20), rng.Intn(20))
			if len(noteIDs) > 0 && rng.Intn(3) == 0 {
				extra += "links:\n  - " + noteIDs[rng.Intn(len(noteIDs))] + "\n"
			}
			if len(noteIDs) > 0 && rng.Intn(4) == 0 {
				extra += "bears_on:\n  - " + noteIDs[rng.Intn(len(noteIDs))] + "\n"
			}
			claim := words[rng.Intn(len(words))] + " " + words[rng.Intn(len(words))] + " claim " + words[rng.Intn(len(words))]
			put("notes/"+id+".md", fmt.Sprintf("---\nid: %s\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nclaim: %s\nstance: crafts\nsource: %s\nevidence: %s\n%s---\n%s", id, claim, sid, ev, extra, vault.NoteBody(claim, ev)))
			noteIDs = append(noteIDs, id)
		}
	}
	for h := 0; h < 20; h++ {
		id := fmt.Sprintf("hunch-%02d", h)
		put("hunches/"+id+".md", fmt.Sprintf("---\nid: %s\ntype: hunch\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nquestion: what about %s %s\nstate: open\nbears_on:\n  - %s\n---\n", id, words[rng.Intn(len(words))], words[rng.Intn(len(words))], noteIDs[rng.Intn(len(noteIDs))]))
	}

	t0 := time.Now()
	rc, m, s := runCmd(d, "verify")
	dv := time.Since(t0)
	t.Logf("verify: %v", dv)
	if rc != 0 {
		t.Fatalf("verify rc=%d %.2000s", rc, s)
	}
	if ps, _ := m["problems"].([]any); len(ps) != 0 {
		t.Fatalf("verify problems: %v", ps[:min(len(ps), 5)])
	}
	if dv > 5*time.Second {
		t.Fatalf("verify too slow: %v", dv)
	}

	query := strings.Join(strings.Fields(strings.Join(lines0(noteIDs, words), " "))[:3], " ")
	t0 = time.Now()
	rc, m, s = runCmd(d, "search", query, "--limit", "10")
	ds := time.Since(t0)
	t.Logf("search %q: %v", query, ds)
	if rc != 0 {
		t.Fatalf("search rc=%d %.2000s", rc, s)
	}
	if hits, _ := m["hits"].([]any); len(hits) < 1 {
		t.Fatalf("search returned no hits: %.500s", s)
	}
	if ds > 5*time.Second {
		t.Fatalf("search too slow: %v", ds)
	}
}

// lines0 returns a deterministic 3-word query drawn from the generated vocabulary.
func lines0(_ []string, words []string) []string {
	return []string{"claim", words[len(words)-3], words[len(words)-2]}
}
