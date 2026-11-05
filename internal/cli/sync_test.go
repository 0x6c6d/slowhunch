package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGitEnv gives git an empty config and a fixed identity.
func isolateGitEnv(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.invalid",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.invalid",
		"SLOWHUNCH_VAULT": "",
	} {
		t.Setenv(k, v)
	}
}

// syncPair returns a bare remote and two fake-model vault clones a and b,
// both tracking origin.
func syncPair(t *testing.T) (bare, a, b string) {
	t.Helper()
	isolateGitEnv(t)
	root := t.TempDir()
	bare = filepath.Join(root, "bare.git")
	gitOut(t, root, "init", "-q", "--bare", bare)
	a = newFakeVault(t)
	gitOut(t, a, "remote", "add", "origin", bare)
	gitOut(t, a, "push", "-q", "-u", "origin", "HEAD")
	b = filepath.Join(root, "b")
	gitOut(t, root, "clone", "-q", bare, b)
	return bare, a, b
}

func headOf(t *testing.T, d string) string {
	t.Helper()
	return strings.TrimSpace(gitOut(t, d, "rev-parse", "HEAD"))
}

func gitPulled(t *testing.T, m map[string]any) float64 {
	t.Helper()
	g, ok := m["git"].(map[string]any)
	if !ok {
		t.Fatalf("no git object: %v", m)
	}
	n, ok := g["pulled"].(float64)
	if !ok {
		t.Fatalf("no git.pulled: %v", g)
	}
	return n
}

func runFlags(d string, global []string, args ...string) (int, map[string]any, string) {
	return runCmd(d, append(append([]string{}, global...), args...)...)
}

func TestPullBeforeWrite(t *testing.T) {
	_, a, b := syncPair(t)
	// B logs its place requests.
	logPath := filepath.Join(t.TempDir(), "place.log")
	cp := filepath.Join(b, ".slowhunch", "config.json")
	raw, err := os.ReadFile(cp)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	pv := cfg["provider"].(map[string]any)
	pv["command"] = append(pv["command"].([]any), "place-log="+logPath)
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cp, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, b, "commit", "-q", "-am", "log place")

	rc, m, s := runCmd(a, "add", "Glaciers carve deep valleys over many thousand years of slow ice flow.", "--as", "article")
	if rc != 0 {
		t.Fatalf("A add rc=%d %s", rc, s)
	}
	atoms, _ := m["atoms"].([]any)
	if len(atoms) == 0 {
		t.Fatalf("A wrote no atoms: %s", s)
	}
	aID := strings.Trim(atoms[0].(map[string]any)["id"].(string), "[]")
	gitOut(t, a, "push", "-q")

	rc, m, s = runCmd(b, "add", "Glaciers carve deep valleys and slow ice flow shapes mountains.", "--as", "article")
	if rc != 0 {
		t.Fatalf("B add rc=%d %s", rc, s)
	}
	if n := gitPulled(t, m); n != 1 {
		t.Fatalf("pulled=%v want 1: %s", n, s)
	}
	if _, err := os.Stat(filepath.Join(b, "notes", aID+".md")); err != nil {
		t.Fatalf("A's note not in B: %v", err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no place request: %v", err)
	}
	if !strings.Contains(string(logged), "id: "+aID+"\n") {
		t.Fatalf("place request lacks %s:\n%s", aID, logged)
	}
	if n := strings.TrimSpace(gitOut(t, b, "rev-list", "--merges", "--count", "HEAD")); n != "0" {
		t.Fatalf("merge commits: %s", n)
	}
}

func TestPullNoPullFlag(t *testing.T) {
	_, a, b := syncPair(t)
	mustCreate(t, a, "topic", `{"title":"From A"}`)
	gitOut(t, a, "push", "-q")
	rc, m, s := runFlags(b, []string{"--no-pull"}, "create", "topic", "--data", `{"title":"From B"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if n := gitPulled(t, m); n != 0 {
		t.Fatalf("pulled=%v", n)
	}
	if _, err := os.Stat(filepath.Join(b, "topics", "from-a.md")); err == nil {
		t.Fatal("--no-pull pulled A's topic")
	}
	gitOut(t, b, "fetch", "-q")
	if n := strings.TrimSpace(gitOut(t, b, "rev-list", "--count", "HEAD..@{u}")); n != "1" {
		t.Fatalf("B behind by %s, want 1", n)
	}
}

func TestPullNoRemote(t *testing.T) {
	isolateGitEnv(t)
	d := newFakeVault(t)
	before := revCount(t, d)
	rc, m, s := runCmd(d, "create", "topic", "--data", `{"title":"Alone"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if n := gitPulled(t, m); n != 0 {
		t.Fatalf("pulled=%v", n)
	}
	if _, ok := m["warnings"]; ok {
		t.Fatalf("unexpected warnings: %s", s)
	}
	if revCount(t, d) != before+1 {
		t.Fatal("no commit")
	}
	rc, m, s = runCmd(d, "add", "Rivers move sediment downstream in floods.", "--as", "article")
	if rc != 0 {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if n := gitPulled(t, m); n != 0 {
		t.Fatalf("add pulled=%v", n)
	}
}

func TestPullFailed(t *testing.T) {
	bare, _, b := syncPair(t)
	if err := os.RemoveAll(bare); err != nil {
		t.Fatal(err)
	}
	rc, m, s := runCmd(b, "create", "topic", "--data", `{"title":"Local"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	ws, _ := m["warnings"].([]any)
	if len(ws) != 1 || ws[0].(map[string]any)["code"] != "PULL_FAILED" {
		t.Fatalf("warnings: %s", s)
	}
	if _, err := os.Stat(filepath.Join(b, "topics", "local.md")); err != nil {
		t.Fatal("topic not written")
	}
	// add reports the warning in errors.
	rc, m, s = runCmd(b, "add", "Wind erodes soft sandstone into arches.", "--as", "article")
	if rc != 0 {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	found := false
	es, _ := m["errors"].([]any)
	for _, e := range es {
		if e.(map[string]any)["code"] == "PULL_FAILED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("add errors lack PULL_FAILED: %s", s)
	}
}

func TestPullFailedRequire(t *testing.T) {
	bare, _, b := syncPair(t)
	cp := filepath.Join(b, ".slowhunch", "config.json")
	raw, err := os.ReadFile(cp)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	g, _ := cfg["git"].(map[string]any)
	if g == nil {
		g = map[string]any{}
	}
	g["require_pull"] = true
	cfg["git"] = g
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cp, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, b, "commit", "-q", "-am", "require pull")
	if err := os.RemoveAll(bare); err != nil {
		t.Fatal(err)
	}
	head := headOf(t, b)
	rc, m, s := runCmd(b, "create", "topic", "--data", `{"title":"Blocked"}`)
	if rc != 1 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if e, _ := m["error"].(map[string]any); e == nil || e["code"] != "PULL_FAILED" {
		t.Fatalf("error: %s", s)
	}
	if _, err := os.Stat(filepath.Join(b, "topics", "blocked.md")); err == nil {
		t.Fatal("topic written")
	}
	rc, _, s = runCmd(b, "add", "Wind erodes soft sandstone into arches.", "--as", "article")
	if rc != 1 || !strings.Contains(s, "PULL_FAILED") {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if headOf(t, b) != head {
		t.Fatal("HEAD moved")
	}
	if st := gitOut(t, b, "status", "--porcelain"); st != "" {
		t.Fatalf("tree changed: %s", st)
	}
	if ents, _ := os.ReadDir(filepath.Join(b, "sources")); len(ents) != 0 {
		t.Fatal("source written")
	}
}

func treeSnapshot(t *testing.T, d string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(d, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[p] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRebaseConflictCLI(t *testing.T) {
	_, a, b := syncPair(t)
	mustCreate(t, a, "topic", `{"title":"Rivers","body":"from A"}`)
	gitOut(t, a, "push", "-q")
	rc, _, s := runFlags(b, []string{"--no-pull"}, "create", "topic", "--data", `{"title":"Rivers","body":"from B"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	head := headOf(t, b)
	before := treeSnapshot(t, b)
	rc, m, s := runCmd(b, "create", "topic", "--data", `{"title":"Other"}`)
	if rc != 4 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	e, _ := m["error"].(map[string]any)
	if e == nil || e["code"] != "CONFLICT" || !strings.Contains(e["message"].(string), "topics/rivers.md") {
		t.Fatalf("error: %s", s)
	}
	if headOf(t, b) != head {
		t.Fatal("HEAD moved")
	}
	after := treeSnapshot(t, b)
	if len(after) != len(before) {
		t.Fatalf("tree changed: %d files, was %d", len(after), len(before))
	}
	for p, c := range before {
		if after[p] != c {
			t.Fatalf("%s changed", p)
		}
	}
	for _, r := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(b, ".git", r)); err == nil {
			t.Fatalf("rebase in progress: %s", r)
		}
	}
}
