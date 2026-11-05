package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitMinimal(t *testing.T) {
	d := filepath.Join(t.TempDir(), "v")
	var out, errb bytes.Buffer
	if rc := Run([]string{"init", d, "--json"}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	if !strings.Contains(out.String(), `"schema": 1`) {
		t.Fatal(out.String())
	}
	for _, f := range []string{"notes", "hunches", "sources", "topics", "proposals", "inbox", "archive", ".slowhunch"} {
		if st, err := os.Stat(filepath.Join(d, f)); err != nil || !st.IsDir() {
			t.Fatalf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(d, ".slowhunch", "config.json")); err != nil {
		t.Fatal(err)
	}
	if rc := Run([]string{"init", "--json"}, strings.NewReader(""), &out, &errb); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
}

func runInit(t *testing.T, d string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := Run([]string{"init", d, "--json"}, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

func gitOut(t *testing.T, d string, args ...string) string {
	t.Helper()
	b, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return string(b)
}

func TestInitCreates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	d := filepath.Join(t.TempDir(), "v")
	rc, out, e := runInit(t, d)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, e)
	}
	for _, f := range []string{"notes", "hunches", "sources", "topics", "proposals", "inbox", "archive", ".slowhunch", ".slowhunch/prompts"} {
		if st, err := os.Stat(filepath.Join(d, f)); err != nil || !st.IsDir() {
			t.Fatalf("missing %s", f)
		}
	}
	a, _ := os.ReadFile(filepath.Join(d, "AGENTS.md"))
	c, _ := os.ReadFile(filepath.Join(d, "CLAUDE.md"))
	if len(a) == 0 || !bytes.Equal(a, c) {
		t.Fatal("agent files differ")
	}
	gi, _ := os.ReadFile(filepath.Join(d, ".gitignore"))
	for _, l := range []string{".slowhunch/cache/", ".slowhunch/audit.log", ".slowhunch/state.json", ".slowhunch/reports/"} {
		if !strings.Contains("\n"+string(gi), "\n"+l+"\n") {
			t.Fatalf("gitignore lacks %s", l)
		}
	}
	if n := strings.TrimSpace(gitOut(t, d, "rev-list", "--count", "HEAD")); n != "1" {
		t.Fatalf("commits=%s", n)
	}
	files := gitOut(t, d, "show", "--name-only", "--format=", "HEAD")
	for _, f := range []string{"AGENTS.md", "CLAUDE.md", ".gitignore", ".slowhunch/config.json",
		".slowhunch/prompts/gate.md", ".slowhunch/prompts/extract.md", ".slowhunch/prompts/place.md"} {
		if !strings.Contains("\n"+files, "\n"+f+"\n") {
			t.Fatalf("commit lacks %s: %s", f, files)
		}
	}
	if !strings.Contains(out, `"warnings": []`) {
		t.Fatal(out)
	}
}

func TestInitTwice(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	d := filepath.Join(t.TempDir(), "v")
	runInit(t, d)
	before, _ := os.ReadFile(filepath.Join(d, "AGENTS.md"))
	rc, out, e := runInit(t, d)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, e)
	}
	if strings.Count(out, `"EXISTS"`) != 2 {
		t.Fatal(out)
	}
	for _, f := range []string{"AGENTS.md", "CLAUDE.md"} {
		after, _ := os.ReadFile(filepath.Join(d, f))
		if !bytes.Equal(before, after) {
			t.Fatalf("%s changed", f)
		}
	}
	if n := strings.TrimSpace(gitOut(t, d, "rev-list", "--count", "HEAD")); n != "1" {
		t.Fatalf("commits=%s", n)
	}
}

func TestInitKeepsForeignAgentsFile(t *testing.T) {
	d := filepath.Join(t.TempDir(), "v")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("my own rules\n")
	if err := os.WriteFile(filepath.Join(d, "AGENTS.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, e := runInit(t, d)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, e)
	}
	got, _ := os.ReadFile(filepath.Join(d, "AGENTS.md"))
	if !bytes.Equal(got, custom) || !strings.Contains(out, `"EXISTS"`) {
		t.Fatal(out)
	}
}

func TestInitGitMissing(t *testing.T) {
	t.Setenv("PATH", "")
	d := filepath.Join(t.TempDir(), "v")
	rc, out, e := runInit(t, d)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, e)
	}
	if !strings.Contains(out, "GIT_MISSING") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
		t.Fatal(".git created")
	}
}

func TestInitProtected(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("SLOWHUNCH_TEST", "1")
	t.Setenv("SLOWHUNCH_PROTECTED", parent)
	d := filepath.Join(parent, "v")
	rc, _, _ := runInit(t, d)
	if rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(d); err == nil {
		t.Fatal("created")
	}
}
