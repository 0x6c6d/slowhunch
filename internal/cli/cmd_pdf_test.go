package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pdfMarker = "slowhunch-pdf-helper"

const pdfHelperText = "Sleep matters for memory. Studies show consolidation happens overnight.\n"

// init turns the test binary into a fake PDF extractor when re-exec'd with pdfMarker.
func init() {
	if len(os.Args) >= 2 && os.Args[1] == pdfMarker {
		fmt.Print(pdfHelperText)
		os.Exit(0)
	}
}

// setPDFCommand writes pdf.command into the vault config and commits it.
func setPDFCommand(t *testing.T, d string, cmd []string) {
	t.Helper()
	p := filepath.Join(d, ".slowhunch", "config.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["pdf"] = map[string]any{"command": cmd}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "add", "-A")
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "pdf command")
}

func TestPDFAdd(t *testing.T) {
	d := newFakeVault(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setPDFCommand(t, d, []string{exe, pdfMarker, "{file}"})
	pdf := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, m, s := runAdd(d, "", pdf, "--title", "T", "--author", "A")
	if rc != 0 || m["ok"] != true {
		t.Fatalf("rc=%d %s", rc, s)
	}
	es, _ := os.ReadDir(filepath.Join(d, "sources"))
	if len(es) != 1 {
		t.Fatalf("sources: %v", es)
	}
	src, err := os.ReadFile(filepath.Join(d, "sources", es[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\nmedium: article\n") {
		t.Fatalf("source not an article:\n%s", src)
	}
	id := strings.TrimSuffix(es[0].Name(), ".md")
	inbox, err := os.ReadFile(filepath.Join(d, "inbox", id+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(inbox), strings.TrimSpace(pdfHelperText)) {
		t.Fatalf("inbox text %q", inbox)
	}
	filepath.WalkDir(d, func(p string, de fs.DirEntry, err error) error {
		if err == nil && strings.EqualFold(filepath.Ext(p), ".pdf") {
			t.Errorf("pdf file in vault: %s", p)
		}
		return nil
	})
}
