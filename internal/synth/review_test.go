package synth

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func hashTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, rel+" "+hex.EncodeToString(s[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hunch(id, state, created, evidence string) string {
	return "---\nid: " + id + "\ntype: hunch\nquestion: q\nstate: " + state + "\ncreated: " + created + "\n" + evidence + "---\n"
}

func note(id, source, extra, body string) string {
	s := "---\nid: " + id + "\ntype: note\nclaim: c\n"
	if source != "" {
		s += "source: " + source + "\n"
	}
	return s + extra + "---\n" + body + "\n"
}

func TestReviewQueries(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"hunches/h-stale.md":   hunch("h-stale", "open", "2026-01-01", ""),
		"hunches/h-fresh.md":   hunch("h-fresh", "warming", "2026-01-01", "evidence:\n  - date: 2026-10-30\n    note: n\n    relation: supports\n"),
		"hunches/h-tested.md":  hunch("h-tested", "tested", "2026-01-01", ""),
		"hunches/h-audited.md": hunch("h-audited", "open", "2026-01-01", ""),
		"sources/s-poem.md":    "---\nid: s-poem\ntype: source\nmedium: poem\n---\n",
		"sources/s-paper.md":   "---\nid: s-paper\ntype: source\nmedium: paper\n---\n",
		"notes/n-orphan.md":    note("n-orphan", "", "", "alone"),
		"notes/n-linked.md":    note("n-linked", "", "", "see [[n-orphan2]]"),
		"notes/n-orphan2.md":   note("n-orphan2", "s-paper", "", "x"),
		"notes/a-rereads.md":   note("a-rereads", "s-poem", "rereads:\n  - n-orphan2\n", "x"),
		"notes/a-echoed.md":    note("a-echoed", "s-poem", "", "x"),
		"notes/a-unplaced.md":  note("a-unplaced", "s-poem", "", "x"),
	}
	for p, c := range files {
		if err := v.WriteFile(p, []byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.Audit(vault.AuditEntry{Time: "2026-11-01T10:00:00Z", Command: "revisit", Paths: []string{"hunches/h-audited.md"}}); err != nil {
		t.Fatal(err)
	}
	log, _ := v.Abs(".slowhunch/audit.log")
	f, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("not json\n")
	f.Close()
	rp := report.New("run1")
	rp.Echoes = append(rp.Echoes, report.Echo{Atom: "[[a-echoed]]", Echoes: "[[n-linked]]", Cosine: 0.9})
	if err := rp.Save(v); err != nil {
		t.Fatal(err)
	}
	ix, lerrs, err := v.LoadIndex()
	if err != nil || len(lerrs) != 0 {
		t.Fatalf("index: %v %v", err, lerrs)
	}

	before := hashTree(t, dir)
	today := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	got, err := RunReview(v, ix, today, 30)
	if err != nil {
		t.Fatal(err)
	}
	if after := hashTree(t, dir); after != before {
		t.Fatal("vault changed during review")
	}
	want := Review{
		StaleHunches: []string{"h-stale"},
		Orphans:      []string{"n-orphan"},
		Unplaced:     []string{"a-unplaced"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
