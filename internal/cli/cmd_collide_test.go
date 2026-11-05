package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

var collideSeed = map[string]string{
	"sources/src-paper.md": "---\nid: src-paper\ntype: source\ncreated: 2026-01-01\nupdated: 2026-01-01\nadded_by: human\nrev: 000000\nmedium: paper\ntitle: Rivers\nauthor: anon\n---\n",
	"sources/src-blog.md":  "---\nid: src-blog\ntype: source\ncreated: 2026-01-01\nupdated: 2026-01-01\nadded_by: human\nrev: 000000\nmedium: blog\ntitle: Skimming\nauthor: anon\n---\n",
	"sources/src-poem.md":  "---\nid: src-poem\ntype: source\ncreated: 2026-01-01\nupdated: 2026-01-01\nadded_by: human\nrev: 000000\nmedium: poem\ntitle: Moon\nauthor: anon\n---\n",
	"notes/n-rivers.md":    "---\nid: n-rivers\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nrev: 000000\nclaim: Rivers carve canyons slowly\nevidence: erosion data\nstance: asserts\nsource: src-paper\nstatus: active\n---\nGranite erodes over epochs.\n",
	"notes/n-skim.md":      "---\nid: n-skim\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nrev: 000000\nclaim: Readers skim headlines first\nevidence: eye tracking\nstance: reads\nsource: src-blog\nstatus: active\n---\nBrowsers glance quickly.\n",
	"notes/n-moon.md":      "---\nid: n-moon\ntype: note\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nrev: 000000\nclaim: Moonlight pools silver\nevidence: moonlight pools silver\nstance: crafts\nlocator: line 1\nsource: src-poem\nstatus: active\n---\nQuiet lake shimmer.\n",
	"hunches/h-sleep.md":   "---\nid: h-sleep\ntype: hunch\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nrev: 000000\nquestion: \"Does napping boost recall?\"\nstate: open\n---\n",
}

// collideVault is a fake-provider vault seeded with notes in different media,
// an aesthetic note and an open hunch; extra args go to the fake model.
func collideVault(t *testing.T, fakeArgs ...string) string {
	t.Helper()
	d := newFakeVault(t)
	for p, c := range collideSeed {
		if err := os.WriteFile(filepath.Join(d, p), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(fakeArgs) > 0 {
		p := filepath.Join(d, ".slowhunch", "config.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		exe, _ := os.Executable()
		cfg["provider"] = map[string]any{"name": "exec", "command": append([]string{exe, "slowhunch-fake-model"}, fakeArgs...)}
		out, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitOut(t, d, "add", "-A")
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "seed")
	return d
}

func seedBytes(t *testing.T, d string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, sub := range []string{"notes", "hunches", "sources"} {
		es, _ := os.ReadDir(filepath.Join(d, sub))
		for _, e := range es {
			b, err := os.ReadFile(filepath.Join(d, sub, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			out[sub+"/"+e.Name()] = string(b)
		}
	}
	return out
}

func proposalFiles(t *testing.T, d string) []string {
	t.Helper()
	es, _ := os.ReadDir(filepath.Join(d, "proposals"))
	out := []string{}
	for _, e := range es {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func readProposal(t *testing.T, d, name string) *frontmatter.Doc {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d, "proposals", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := frontmatter.Parse(b)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if err := vault.Validate(doc, strings.TrimSuffix(name, ".md")); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return doc
}

func countsOf(m map[string]any) map[string]float64 {
	out := map[string]float64{}
	c, _ := m["counts"].(map[string]any)
	for k, v := range c {
		out[k], _ = v.(float64)
	}
	return out
}

func TestCollideRun(t *testing.T) {
	d := collideVault(t)
	before := seedBytes(t, d)
	commits := addCommits(t, d)
	rc, m, s := runCmd(d, "collide", "--limit", "3")
	if rc != 0 {
		t.Fatal(s)
	}
	c := countsOf(m)
	if c["pairs_candidates"] != 6 || c["pairs_tried"] != 3 || c["pairs_kept"] != 3 {
		t.Fatalf("counts %v", c)
	}
	if m["capped"] != true {
		t.Fatalf("capped missing: %s", s)
	}
	capWarn := false
	for _, e := range batchErrors(m) {
		if e["item"] == "collide" && e["code"] == "CAP_REACHED" && e["reason"] == "max_pairs 3 of 6 candidates" {
			capWarn = true
		}
	}
	if !capWarn {
		t.Fatalf("no cap warning: %s", s)
	}
	files := proposalFiles(t, d)
	if !reflect.DeepEqual(files, []string{"fake-idea-2.md", "fake-idea-3.md", "fake-idea.md"}) {
		t.Fatalf("proposals %v", files)
	}
	cols, _ := m["collisions"].([]any)
	if len(cols) != 3 {
		t.Fatalf("collisions %s", s)
	}
	for _, x := range cols {
		col := x.(map[string]any)
		items := col["items"].([]any)
		prop := strings.TrimSuffix(strings.TrimPrefix(col["proposal"].(string), "[["), "]]")
		doc := readProposal(t, d, prop+".md")
		got := doc.GetList("items")
		if len(items) != 2 || items[0] != "[["+got[0]+"]]" || items[1] != "[["+got[1]+"]]" {
			t.Fatalf("items mismatch %v %v", items, got)
		}
		if doc.GetString("status") != "open" || doc.GetString("idea") != "fake idea" || doc.GetString("cheapest_test") != "fake test" {
			t.Fatalf("doc %s", doc.Bytes())
		}
		if !strings.HasPrefix(doc.GetString("author"), "agent:exec/") {
			t.Fatalf("author %q", doc.GetString("author"))
		}
		var wantBears []string
		for _, id := range got {
			if id == "h-sleep" {
				wantBears = append(wantBears, id)
			}
		}
		if bears := doc.GetList("bears_on"); len(bears) != len(wantBears) || (len(bears) == 1 && bears[0] != "h-sleep") {
			t.Fatalf("bears_on %v for %v", bears, got)
		}
	}
	cost, _ := m["cost"].(map[string]any)
	if _, ok := cost["collide"]; !ok {
		t.Fatalf("cost %v", cost)
	}
	if n := addCommits(t, d); n != commits+1 {
		t.Fatalf("commits %d -> %d", commits, n)
	}
	if rf := reportFiles(t, d); len(rf) != 1 {
		t.Fatalf("reports %v", rf)
	}
	if !reflect.DeepEqual(before, seedBytes(t, d)) {
		t.Fatal("notes, hunches or sources changed")
	}
	if rc, _, s := runCmd(d, "lint"); rc != 0 {
		t.Fatalf("lint after collide: %s", s)
	}
	if st := gitOut(t, d, "status", "--porcelain", "--", "proposals", "notes", "hunches"); st != "" {
		t.Fatalf("uncommitted: %s", st)
	}
}

func TestCollideRerun(t *testing.T) {
	d := collideVault(t)
	rc, m, s := runCmd(d, "collide")
	if rc != 0 {
		t.Fatal(s)
	}
	if c := countsOf(m); c["pairs_kept"] != 6 || m["capped"] != false {
		t.Fatalf("first run %s", s)
	}
	files := proposalFiles(t, d)
	commits := addCommits(t, d)
	tree := reviewTree(t, d)
	rc, m, s = runCmd(d, "collide")
	if rc != 0 {
		t.Fatal(s)
	}
	if c := countsOf(m); c["pairs_candidates"] != 0 || c["pairs_tried"] != 0 || c["pairs_kept"] != 0 {
		t.Fatalf("rerun counts %s", s)
	}
	if addCommits(t, d) != commits || !reflect.DeepEqual(files, proposalFiles(t, d)) || reviewTree(t, d) != tree {
		t.Fatal("rerun changed the vault")
	}
}

func TestCollideDryRun(t *testing.T) {
	d := collideVault(t)
	// A provider that always fails: any model call would surface as an error.
	p := filepath.Join(d, ".slowhunch", "config.json")
	raw, _ := os.ReadFile(p)
	var cfg map[string]any
	_ = json.Unmarshal(raw, &cfg)
	cfg["provider"] = map[string]any{"name": "exec", "command": []string{"false"}}
	out, _ := json.Marshal(cfg)
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "cfg")
	tree := reviewTree(t, d)
	rc, m, s := runCmd(d, "collide", "--dry-run", "--limit", "2")
	if rc != 0 {
		t.Fatal(s)
	}
	pairs, _ := m["pairs"].([]any)
	if len(pairs) != 2 || m["capped"] != true {
		t.Fatalf("dry-run output %s", s)
	}
	if c := countsOf(m); c["pairs_candidates"] != 6 || c["pairs_tried"] != 0 {
		t.Fatalf("dry-run counts %s", s)
	}
	if reviewTree(t, d) != tree || len(proposalFiles(t, d)) != 0 || len(reportFiles(t, d)) != 0 {
		t.Fatal("dry-run changed the vault")
	}
}

func TestCollideHostileReply(t *testing.T) {
	long := strings.Repeat("a", 250)
	replies := map[string]string{
		"n-rivers n-skim":  `{"idea": "../../etc/passwd x", "cheapest_test": "try it", "keep": true}`,
		"n-moon n-rivers":  `{"idea": "` + long + `", "cheapest_test": "try it", "keep": true}`,
		"h-sleep n-rivers": `{"idea": "fine idea", "cheapest_test": "   ", "keep": true}`,
		"n-moon n-skim":    `{"idea": "leak sk-abcdefghijklmnopqrstuv", "cheapest_test": "try it", "keep": true}`,
		"h-sleep n-moon":   `{"idea": "nothing", "cheapest_test": "none", "keep": false}`,
	}
	rf := filepath.Join(t.TempDir(), "replies.json")
	b, _ := json.Marshal(replies)
	if err := os.WriteFile(rf, b, 0o644); err != nil {
		t.Fatal(err)
	}
	d := collideVault(t, "collide-replies="+rf)
	before := seedBytes(t, d)
	rc, m, s := runCmd(d, "collide")
	if rc != 0 {
		t.Fatal(s)
	}
	files := proposalFiles(t, d)
	if !reflect.DeepEqual(files, []string{"etc-passwd-x.md", "fake-idea.md"}) {
		t.Fatalf("proposals %v", files)
	}
	doc := readProposal(t, d, "etc-passwd-x.md")
	if got := doc.GetList("items"); !reflect.DeepEqual(got, []string{"n-rivers", "n-skim"}) {
		t.Fatalf("items %v", got)
	}
	for _, bad := range []string{filepath.Join(d, "etc"), filepath.Join(filepath.Dir(d), "etc")} {
		if _, err := os.Stat(bad); err == nil {
			t.Fatalf("%s exists", bad)
		}
	}
	errItems := map[string]bool{}
	for _, e := range batchErrors(m) {
		errItems[e["item"].(string)] = true
		if strings.Contains(e["reason"].(string), "sk-abc") {
			t.Fatalf("secret echoed: %v", e)
		}
	}
	for _, it := range []string{"n-moon+n-rivers", "h-sleep+n-rivers", "n-moon+n-skim"} {
		if !errItems[it] {
			t.Fatalf("missing error for %s: %s", it, s)
		}
	}
	if c := countsOf(m); c["pairs_tried"] != 6 || c["pairs_kept"] != 2 {
		t.Fatalf("counts %s", s)
	}
	if strings.Contains(s, "sk-abcdefghijklmnopqrstuv") {
		t.Fatal("secret in output")
	}
	if !reflect.DeepEqual(before, seedBytes(t, d)) {
		t.Fatal("notes, hunches or sources changed")
	}
}
