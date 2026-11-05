package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixClock(t *testing.T, s string) {
	t.Helper()
	old := nowFunc
	tm, _ := time.Parse("2006-01-02", s)
	nowFunc = func() time.Time { return tm }
	t.Cleanup(func() { nowFunc = old })
}

func addHunch(t *testing.T, d string, args ...string) string {
	t.Helper()
	rc, m, s := runCmd(d, append([]string{"hunch", "add"}, args...)...)
	if rc != 0 {
		t.Fatalf("hunch add rc=%d %s", rc, s)
	}
	return m["id"].(string)
}

func TestHunchAdd(t *testing.T) {
	d := newVault(t)
	rc, m, s := runCmd(d, "hunch", "add", "Why do lines stick?", "--next-test", "recite", "--revisit", "2026-12-01", "--tags", "a,b")
	if rc != 0 {
		t.Fatal(s)
	}
	if m["id"] != "why-do-lines-stick" || m["path"] != "hunches/why-do-lines-stick.md" {
		t.Fatalf("%v", m)
	}
	b, err := os.ReadFile(filepath.Join(d, "hunches/why-do-lines-stick.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(b), "state: open") || !contains(string(b), "next_test") {
		t.Fatalf("%s", b)
	}
	if id2 := addHunch(t, d, "Why do lines stick?"); id2 != "why-do-lines-stick-2" {
		t.Fatal(id2)
	}
	if rc, _, _ := runCmd(d, "hunch", "add", "  "); rc != 2 {
		t.Fatalf("empty question rc=%d", rc)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestHunchEvidence(t *testing.T) {
	fixClock(t, "2026-11-05")
	d := newVault(t)
	h := addHunch(t, d, "Why do lines stick")
	n := mustCreate(t, d, "note", `{"claim":"c","evidence":"e","stance":"asserts"}`)["id"].(string)
	rc, m, s := runCmd(d, "hunch", "evidence", h, "--note", n, "--relation", "supports")
	if rc != 0 {
		t.Fatal(s)
	}
	if m["state"] != "warming" || m["evidence_count"] != float64(1) {
		t.Fatalf("%v", m)
	}
	p := filepath.Join(d, "hunches", h+".md")
	b, _ := os.ReadFile(p)
	if !contains(string(b), "date: 2026-11-05") && !contains(string(b), `"2026-11-05"`) {
		t.Fatalf("no dated entry: %s", b)
	}
	before, _ := os.ReadFile(p)
	if rc, _, _ := runCmd(d, "hunch", "evidence", h, "--note", "nope", "--relation", "supports"); rc == 0 {
		t.Fatal("unknown note accepted")
	}
	if _, m, _ := runCmd(d, "hunch", "evidence", h, "--note", "nope", "--relation", "supports"); m["error"] == nil {
		t.Logf("%v", m)
	}
	if rc, _, _ := runCmd(d, "hunch", "evidence", h, "--note", n, "--relation", "proves"); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("file changed")
	}
}

func TestHunchState(t *testing.T) {
	d := newVault(t)
	h := addHunch(t, d, "Q one")
	if rc, m, s := runCmd(d, "hunch", "state", h, "tested"); rc != 0 || m["state"] != "tested" {
		t.Fatal(s)
	}
	b, _ := os.ReadFile(filepath.Join(d, "hunches", h+".md"))
	if !contains(string(b), "state: tested") {
		t.Fatalf("%s", b)
	}
	if rc, _, _ := runCmd(d, "hunch", "state", h, "bogus"); rc != 2 {
		t.Fatal("bad state")
	}
	if rc, _, _ := runCmd(d, "hunch", "state", "missing", "open"); rc == 0 {
		t.Fatal("missing")
	}
}

func TestHunchDue(t *testing.T) {
	fixClock(t, "2026-11-05")
	d := newVault(t)
	addHunch(t, d, "Past one", "--revisit", "2026-11-01")
	addHunch(t, d, "Future one", "--revisit", "2026-12-01")
	dr := addHunch(t, d, "Dropped one", "--revisit", "2026-10-01")
	addHunch(t, d, "No revisit")
	runCmd(d, "hunch", "state", dr, "dropped")
	rc, m, s := runCmd(d, "hunch", "due")
	if rc != 0 {
		t.Fatal(s)
	}
	due, _ := m["due"].([]any)
	if len(due) != 1 || due[0].(map[string]any)["id"] != "past-one" || m["today"] != "2026-11-05" {
		t.Fatalf("%v", m)
	}
}
