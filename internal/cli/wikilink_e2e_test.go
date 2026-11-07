package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wikilinkSrc = "Both overlap with [[bloodhound]] for AD enumeration and attack paths.\n"

// wikilinkVault is a fake-provider vault whose extract returns evidence ev.
func wikilinkVault(t *testing.T, ev string) (string, string) {
	d := newFakeVault(t)
	p := filepath.Join(d, ".slowhunch", "config.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	prov := cfg["provider"].(map[string]any)
	prov["command"] = append(prov["command"].([]any), "evidence="+ev)
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "add", "-A")
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "evidence override")
	src := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(src, []byte(wikilinkSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return d, src
}

func TestAddWikilinkVerifyClean(t *testing.T) {
	d, src := wikilinkVault(t, "Both overlap with bloodhound for AD enumeration and attack paths.")
	rc, m, s := runAdd(d, "", src, "--as", "article")
	if rc != 0 || m["ok"] != true {
		t.Fatalf("rc=%d %s", rc, s)
	}
	inbox, _ := filepath.Glob(filepath.Join(d, "inbox", "*.txt"))
	if len(inbox) != 1 {
		t.Fatalf("inbox files: %v", inbox)
	}
	b, _ := os.ReadFile(inbox[0])
	if strings.Contains(string(b), "[[") || !strings.Contains(string(b), "bloodhound") {
		t.Fatalf("inbox: %q", b)
	}
	notes, _ := filepath.Glob(filepath.Join(d, "notes", "*.md"))
	if len(notes) != 1 {
		t.Fatalf("notes: %v", notes)
	}
	nb, _ := os.ReadFile(notes[0])
	if strings.Contains(string(nb), "[[") || !strings.Contains(string(nb), "evidence: Both overlap with bloodhound") {
		t.Fatalf("note: %q", nb)
	}
	rc, _, s = runCmd(d, "verify")
	if rc != 0 || strings.Contains(s, "DANGLING_LINK") {
		t.Fatalf("verify rc=%d %s", rc, s)
	}
}

func TestAddRawWikilinkEvidenceRejected(t *testing.T) {
	d, src := wikilinkVault(t, "Both overlap with [[bloodhound]] for AD enumeration and attack paths.")
	rc, _, s := runAdd(d, "", src, "--as", "article")
	if !strings.Contains(s, "EVIDENCE_NOT_FOUND") && !strings.Contains(s, "rejected") {
		t.Fatalf("expected evidence rejection: rc=%d %s", rc, s)
	}
	if n := countFiles(t, d, "notes"); n != 0 {
		t.Fatalf("note written (%d): rc=%d %s", n, rc, s)
	}
	t.Logf("rc=%d %s", rc, s)
}
