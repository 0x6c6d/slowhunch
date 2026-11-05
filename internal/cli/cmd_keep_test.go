package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func TestKeepNamed(t *testing.T) {
	d := newVault(t)
	tp := mustCreate(t, d, "topic", `{"title":"Rivers","body":"water flows"}`)
	h := mustCreate(t, d, "hunch", `{"question":"Why?"}`)["id"].(string)
	before := commitCount(t, d)
	rc, r, s := runCmd(d, "keep", "rivers carve valleys", "--ids", h+",rivers", "--topic", "rivers")
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	got := readVaultFile(t, d, "topics/rivers.md")
	want := "water flows\n\n### " + time.Now().Format("2006-01-02") + "\nrivers carve valleys\n[[" + h + "]] [[rivers]]\n"
	if !strings.HasSuffix(got, want) {
		t.Fatalf("%q", got)
	}
	doc, err := frontmatter.Parse([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	if r["rev"] == tp["rev"] || r["rev"] != frontmatter.Rev(doc) || r["id"] != "rivers" || r["path"] != "topics/rivers.md" {
		t.Fatalf("bad result %v", r)
	}
	if commitCount(t, d) != before+1 {
		t.Fatal("not one commit")
	}
}

func TestKeepNearest(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Cooking","body":"pasta sauce and bread"}`)
	mustCreate(t, d, "topic", `{"title":"Rivers","body":"water flows through valleys"}`)
	rc, r, s := runCmd(d, "keep", "valleys and water", "--ids", "cooking")
	if rc != 0 || r["id"] != "rivers" {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if !strings.Contains(readVaultFile(t, d, "topics/rivers.md"), "valleys and water\n[[cooking]]") {
		t.Fatal("not appended")
	}
	if strings.Contains(readVaultFile(t, d, "topics/cooking.md"), "###") {
		t.Fatal("wrong topic changed")
	}
}

func TestKeepFailures(t *testing.T) {
	d := newVault(t)
	empty := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Rivers","body":"water"}`)
	h := mustCreate(t, empty, "hunch", `{"question":"Why?"}`)["id"].(string)
	state := func(dir string) string {
		return gitOut(t, dir, "rev-parse", "HEAD") + gitOut(t, dir, "status", "--porcelain") + readVaultFile(t, dir, "topics/rivers.md")
	}
	secret := "sk-" + strings.Repeat("a", 20)
	cases := []struct {
		name, dir, code string
		args            []string
	}{
		{"missing id", d, "NOT_FOUND", []string{"keep", "x", "--ids", "nope"}},
		{"unknown topic", d, "NOT_FOUND", []string{"keep", "x", "--ids", "rivers", "--topic", "nope"}},
		{"no topics", empty, "NOT_FOUND", []string{"keep", "x", "--ids", h}},
		{"secret", d, "BAD_INPUT", []string{"keep", secret, "--ids", "rivers"}},
		{"too long", d, "TOO_LONG", []string{"keep", strings.Repeat("a", 2001), "--ids", "rivers"}},
	}
	for _, c := range cases {
		var before string
		if c.dir == d {
			before = state(d)
		}
		rc, m, s := runCmd(c.dir, c.args...)
		e, _ := m["error"].(map[string]any)
		if rc == 0 || e == nil || e["code"] != c.code {
			t.Fatalf("%s: rc=%d %s", c.name, rc, s)
		}
		if c.dir == d && state(d) != before {
			t.Fatalf("%s: vault changed", c.name)
		}
	}
	if rc, _, s := runCmd(d, "keep", strings.Repeat("a", 2000), "--ids", "rivers"); rc != 0 {
		t.Fatalf("2000 chars: rc=%d %s", rc, s)
	}
}
