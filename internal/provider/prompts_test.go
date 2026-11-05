package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptsEmbedded(t *testing.T) {
	p, err := LoadPrompts(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"gate": "gate-1", "extract": "extract-1", "place": "place-1"}
	v := Versions(p)
	for r, ver := range want {
		if v[r] != ver {
			t.Errorf("%s version %q", r, v[r])
		}
		if !strings.Contains(p[r].Text, "JSON") || !strings.Contains(p[r].Text, "SLOWHUNCH-INTAKE") {
			t.Errorf("%s body lacks JSON/delimiter", r)
		}
	}
}

func TestPromptsOverride(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".slowhunch", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gate.md"), []byte("---\nrole: gate\nversion: gate-7\n---\ncustom JSON\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPrompts(root)
	if err != nil {
		t.Fatal(err)
	}
	v := Versions(p)
	if v["gate"] != "gate-7" || v["extract"] != "extract-1" || v["place"] != "place-1" {
		t.Fatalf("versions %v", v)
	}
}

func TestPromptsInvalid(t *testing.T) {
	if _, err := ParsePrompt("gate", []byte("---\nrole: gate\n---\nx\n")); err == nil {
		t.Fatal("expected error for missing version")
	}
	if _, err := ParsePrompt("gate", []byte("---\nrole: place\nversion: p-1\n---\nx\n")); err == nil {
		t.Fatal("expected role mismatch error")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".slowhunch", "prompts")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "place.md"), []byte("---\nrole: place\n---\nx\n"), 0o644)
	if _, err := LoadPrompts(root); err == nil {
		t.Fatal("expected error from invalid vault prompt")
	}
}
