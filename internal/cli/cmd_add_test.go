package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runAdd(d, stdin string, args ...string) (int, map[string]any, string) {
	var out, errb bytes.Buffer
	rc := Run(append([]string{"add"}, append(args, "--vault", d)...), strings.NewReader(stdin), &out, &errb)
	m := map[string]any{}
	_ = json.Unmarshal(out.Bytes(), &m)
	return rc, m, out.String() + errb.String()
}

func countFiles(t *testing.T, d, sub string) int {
	t.Helper()
	es, _ := os.ReadDir(filepath.Join(d, sub))
	return len(es)
}

func addCommits(t *testing.T, d string) int {
	return len(strings.Fields(gitOut(t, d, "log", "--format=%H")))
}

func TestAddCLIArticle(t *testing.T) {
	d := newFakeVault(t)
	before := addCommits(t, d)
	rc, m, s := runAdd(d, "Sleep matters for memory. Studies show consolidation happens overnight.\n", "-", "--as", "article")
	if rc != 0 || m["schema"] != float64(1) || m["ok"] != true || m["source"] == nil {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if countFiles(t, d, "sources") == 0 || countFiles(t, d, "notes") == 0 {
		t.Fatalf("missing source/atom: %s", s)
	}
	if n := addCommits(t, d); n != before+1 {
		t.Fatalf("commits %d -> %d", before, n)
	}
}

func TestAddCLIPoem(t *testing.T) {
	d := newFakeVault(t)
	rc, m, s := runAdd(d, "", "I think that I shall never see\nA poem lovely as a tree.", "--as", "poem", "--title", "Trees", "--author", "Philip Larkin")
	if rc != 0 || m["ok"] != true {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if countFiles(t, d, "notes") == 0 {
		t.Fatal(s)
	}
}

func TestAddCLIErrors(t *testing.T) {
	d := newFakeVault(t)
	setPDFCommand(t, d, []string{"slowhunch-no-such-pdf-tool", "{file}"})
	pdf := filepath.Join(t.TempDir(), "p.pdf")
	os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o644)
	rc, _, s := runAdd(d, "", pdf)
	if rc != 2 || !strings.Contains(s, "UNSUPPORTED") || !strings.Contains(s, "install pdftotext or set pdf.command") {
		t.Fatalf("pdf rc=%d %s", rc, s)
	}
	rc, _, s = runAdd(d, strings.Repeat("a", 12001), "-", "--as", "article")
	if rc != 2 || !strings.Contains(s, "TOO_LONG") {
		t.Fatalf("long rc=%d %s", rc, s)
	}
	rc, _, s = runAdd(d, "", "A line of film text", "--as", "film")
	if rc != 2 {
		t.Fatalf("film rc=%d %s", rc, s)
	}
	p := filepath.Join(d, ".slowhunch", "config.json")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, bytes.Replace(raw, []byte(`"exec"`), []byte(`"nope"`), 1), 0o644)
	rc, _, s = runAdd(d, "", "Some text here", "--as", "article")
	if rc != 1 {
		t.Fatalf("provider rc=%d %s", rc, s)
	}
}

func TestAddCLIDryRun(t *testing.T) {
	d := newFakeVault(t)
	before := gitOut(t, d, "status", "--porcelain") + gitOut(t, d, "log", "--format=%H")
	rc, _, s := runAdd(d, "", "Water boils at 100C at sea level.", "--as", "article", "--dry-run")
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if after := gitOut(t, d, "status", "--porcelain") + gitOut(t, d, "log", "--format=%H"); after != before {
		t.Fatalf("tree changed: %s", after)
	}
	if countFiles(t, d, "sources")+countFiles(t, d, "notes")+countFiles(t, d, "inbox") != 0 {
		t.Fatal("files written")
	}
}

func TestAddCLIConflict(t *testing.T) {
	d := newFakeVault(t)
	rc, m, s := runCmd(d, "create", "hunch", "--data", `{"question":"Why sleep?"}`)
	if rc != 0 {
		t.Fatal(s)
	}
	id := m["id"].(string)
	hp := filepath.Join(d, "hunches", id+".md")
	f, err := os.OpenFile(hp, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\ndirty edit\n")
	f.Close()
	rc, _, s = runAdd(d, "", "Sleep consolidates memory during the night.", "--as", "article", "--hunch", id)
	if rc != 4 || !strings.Contains(s, "CONFLICT") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if countFiles(t, d, "sources") != 0 {
		t.Fatal("source written")
	}
}
