package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBatch writes files (name -> content) and the batch file into a temp dir
// and returns the batch file path.
func writeBatch(t *testing.T, files map[string]string, batch string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, "batch.txt")
	if err := os.WriteFile(p, []byte(batch), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func reportFiles(t *testing.T, d string) []string {
	t.Helper()
	es, _ := os.ReadDir(filepath.Join(d, ".slowhunch", "reports"))
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func batchErrors(m map[string]any) []map[string]any {
	var out []map[string]any
	list, _ := m["errors"].([]any)
	for _, x := range list {
		if e, ok := x.(map[string]any); ok {
			out = append(out, e)
		}
	}
	return out
}

func TestBatchAdd(t *testing.T) {
	d := newFakeVault(t)
	setPDFCommand(t, d, []string{"slowhunch-no-such-pdf-tool", "{file}"})
	before := addCommits(t, d)
	files := map[string]string{
		"a.txt":   "Sleep matters for memory. Studies show consolidation happens overnight.\n",
		"b.txt":   "Rivers flood in spring when snow melts quickly in the mountains.\n",
		"c.txt":   "Coffee raises alertness for a few hours after drinking it.\n",
		"p.pdf":   "%PDF-1.4\n",
		"big.txt": strings.Repeat("word ", 3000),
	}
	batch := writeBatch(t, files, "a.txt\nb.txt|https://example.invalid/b|Floods|Ada Lovelace\np.pdf\nc.txt\nbig.txt\n")
	rc, m, s := runAdd(d, "", "--from", batch, "--as", "article")
	if rc != 0 || m["ok"] != true {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if n := countFiles(t, d, "sources"); n != 3 {
		t.Fatalf("sources %d: %s", n, s)
	}
	errsList := batchErrors(m)
	if len(errsList) != 2 {
		t.Fatalf("errors %v", errsList)
	}
	if errsList[0]["item"] != "line 3: p.pdf" || errsList[0]["code"] != "UNSUPPORTED" {
		t.Fatalf("pdf error %v", errsList[0])
	}
	if errsList[1]["item"] != "line 5: big.txt" || errsList[1]["code"] != "TOO_LONG" {
		t.Fatalf("big error %v", errsList[1])
	}
	if n := addCommits(t, d); n != before+1 {
		t.Fatalf("commits %d -> %d", before, n)
	}
	subj := gitOut(t, d, "log", "-1", "--format=%s")
	if !strings.HasPrefix(subj, "slowhunch: add ") {
		t.Fatalf("subject %q", subj)
	}
	if st := gitOut(t, d, "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("dirty after batch: %s", st)
	}
	runID, _ := m["run_id"].(string)
	reps := reportFiles(t, d)
	if len(reps) != 5 {
		t.Fatalf("reports %v", reps)
	}
	for i := 1; i <= 5; i++ {
		if _, err := os.Stat(filepath.Join(d, ".slowhunch", "reports", fmt.Sprintf("%s-%d.json", runID, i))); err != nil {
			t.Fatalf("report %d: %v (%v)", i, err, reps)
		}
	}
	counts, _ := m["counts"].(map[string]any)
	if counts["entries"] != float64(5) || counts["written"] != float64(3) || counts["failed"] != float64(2) {
		t.Fatalf("counts %v", counts)
	}
	items, _ := m["items"].([]any)
	if len(items) != 5 {
		t.Fatalf("items %d", len(items))
	}
	git, _ := m["git"].(map[string]any)
	head := strings.TrimSpace(gitOut(t, d, "rev-parse", "HEAD"))
	if git["commit"] != head {
		t.Fatalf("git %v, head %s", git, head)
	}
	es, _ := os.ReadDir(filepath.Join(d, "sources"))
	foundURL := false
	for _, e := range es {
		b, _ := os.ReadFile(filepath.Join(d, "sources", e.Name()))
		if strings.Contains(string(b), "https://example.invalid/b") && strings.Contains(string(b), "title: Floods") {
			foundURL = true
		}
	}
	if !foundURL {
		t.Fatal("batch line url/title not applied")
	}

	// The same batch again: every entry is existing or failed, nothing new is committed.
	rc, m, s = runAdd(d, "", "--from", batch, "--as", "article")
	if rc != 0 {
		t.Fatalf("rerun rc=%d %s", rc, s)
	}
	if n := countFiles(t, d, "sources"); n != 3 {
		t.Fatalf("rerun sources %d", n)
	}
	if n := addCommits(t, d); n != before+1 {
		t.Fatalf("rerun commits %d -> %d", before, n)
	}
}

func TestBatchTooLong(t *testing.T) {
	d := newVault(t)
	// A provider that must never run: any call would fail the batch differently.
	p := filepath.Join(d, ".slowhunch", "config.json")
	if err := os.WriteFile(p, []byte(`{"provider":{"name":"exec","command":["slowhunch-no-such-model"]}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "add", "-A")
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "cfg")
	files := map[string]string{}
	var b strings.Builder
	for i := 1; i <= 51; i++ {
		name := fmt.Sprintf("e%d.txt", i)
		files[name] = fmt.Sprintf("text number %d about rivers\n", i)
		b.WriteString(name + "\n")
	}
	batch := writeBatch(t, files, b.String())
	before := addCommits(t, d)
	rc, _, s := runAdd(d, "", "--from", batch, "--as", "article")
	if rc != 2 || !strings.Contains(s, `"code": "TOO_LONG"`) {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if n := addCommits(t, d); n != before {
		t.Fatalf("commits %d -> %d", before, n)
	}
	for _, sub := range []string{"sources", "inbox", "notes"} {
		if n := countFiles(t, d, sub); n != 0 {
			t.Fatalf("%s has %d files", sub, n)
		}
	}
	if r := reportFiles(t, d); len(r) != 0 {
		t.Fatalf("reports %v", r)
	}
	if st := gitOut(t, d, "status", "--porcelain"); strings.TrimSpace(st) != "" {
		t.Fatalf("dirty: %s", st)
	}
}

func TestBatchLineFormat(t *testing.T) {
	d := newFakeVault(t)
	files := map[string]string{
		"sub/a.txt": "Sleep matters for memory. Studies show consolidation happens overnight.\n",
		"b.txt":     "Rivers flood in spring when snow melts quickly in the mountains.\n",
	}
	batch := writeBatch(t, files, "# a comment\n\n   \nsub/a.txt\nb.txt|x|y\n  # indented comment\n")
	before := addCommits(t, d)
	rc, m, s := runAdd(d, "", "--from", batch, "--as", "article")
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	counts, _ := m["counts"].(map[string]any)
	if counts["entries"] != float64(2) || counts["written"] != float64(1) || counts["failed"] != float64(1) {
		t.Fatalf("counts %v", counts)
	}
	es := batchErrors(m)
	if len(es) != 1 || es[0]["item"] != "line 5: b.txt" || es[0]["code"] != "BAD_INPUT" {
		t.Fatalf("errors %v", es)
	}
	if n := countFiles(t, d, "sources"); n != 1 {
		t.Fatalf("sources %d", n)
	}
	if n := addCommits(t, d); n != before+1 {
		t.Fatalf("commits %d -> %d", before, n)
	}

	// All entries failing exits non-zero with the first error's code; nothing is committed.
	bad := writeBatch(t, map[string]string{"x.doc": "nope"}, "missing.txt\nx.doc\na|b\n")
	before = addCommits(t, d)
	rc, m, s = runAdd(d, "", "--from", bad, "--as", "article")
	if rc != 2 || m["ok"] != false || !strings.Contains(s, `"code": "NOT_FOUND"`) {
		t.Fatalf("all-failed rc=%d %s", rc, s)
	}
	if es := batchErrors(m); len(es) != 3 {
		t.Fatalf("all-failed errors %v", es)
	}
	if n := addCommits(t, d); n != before {
		t.Fatalf("all-failed commits %d -> %d", before, n)
	}

	// Flag rules.
	for _, args := range [][]string{
		{"--from", batch, "some text"},
		{"--from", batch, "--title", "T"},
		{"--from", batch, "--url", "https://example.invalid"},
		{"--from", batch, "--author", "A"},
	} {
		rc, _, s := runAdd(d, "", args...)
		if rc != 2 || !strings.Contains(s, `"code": "BAD_INPUT"`) {
			t.Fatalf("%v: rc=%d %s", args, rc, s)
		}
	}
}
