package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setGitPush sets git.push in the vault config of d and commits the change.
func setGitPush(t *testing.T, d string, on bool) {
	t.Helper()
	cp := filepath.Join(d, ".slowhunch", "config.json")
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
	g["push"] = on
	cfg["git"] = g
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cp, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "commit", "-q", "-am", "git.push")
}

func gitPushed(t *testing.T, m map[string]any) bool {
	t.Helper()
	g, ok := m["git"].(map[string]any)
	if !ok {
		t.Fatalf("no git object: %v", m)
	}
	b, ok := g["pushed"].(bool)
	if !ok {
		t.Fatalf("no git.pushed: %v", g)
	}
	return b
}

func bareHead(t *testing.T, bare string) string {
	t.Helper()
	return strings.TrimSpace(gitOut(t, bare, "rev-parse", "HEAD"))
}

func hasCode(m map[string]any, key, code string) bool {
	list, _ := m[key].([]any)
	for _, e := range list {
		if em, ok := e.(map[string]any); ok && em["code"] == code {
			return true
		}
	}
	return false
}

func TestPushOptIn(t *testing.T) {
	bare, a, _ := syncPair(t)
	setGitPush(t, a, true)
	rc, m, s := runCmd(a, "create", "topic", "--data", `{"title":"Pushed"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if !gitPushed(t, m) {
		t.Fatalf("pushed=false: %s", s)
	}
	if bareHead(t, bare) != headOf(t, a) {
		t.Fatal("bare remote did not receive the commit")
	}
	// add: the report carries pushed, also in the saved file.
	rc, m, s = runCmd(a, "add", "Glaciers carve deep valleys over many thousand years.", "--as", "article")
	if rc != 0 {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if !gitPushed(t, m) {
		t.Fatalf("add pushed=false: %s", s)
	}
	if bareHead(t, bare) != headOf(t, a) {
		t.Fatal("bare remote did not receive the add commit")
	}
	raw, err := os.ReadFile(filepath.Join(a, ".slowhunch", "reports", m["run_id"].(string)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"pushed": true`) {
		t.Fatalf("saved report lacks pushed: %s", raw)
	}
	if st := gitOut(t, a, "status", "--porcelain"); st != "" {
		t.Fatalf("tree dirty: %s", st)
	}
}

func TestPushDefaultOff(t *testing.T) {
	bare, a, _ := syncPair(t)
	before := bareHead(t, bare)
	rc, m, s := runCmd(a, "create", "topic", "--data", `{"title":"Local only"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if gitPushed(t, m) {
		t.Fatalf("pushed=true: %s", s)
	}
	rc, m, s = runCmd(a, "add", "Rivers move sediment downstream in floods.", "--as", "article")
	if rc != 0 {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if gitPushed(t, m) {
		t.Fatalf("add pushed=true: %s", s)
	}
	if bareHead(t, bare) != before {
		t.Fatal("default config pushed")
	}
}

func TestPushFlags(t *testing.T) {
	bare, a, _ := syncPair(t)
	rc, m, s := runFlags(a, []string{"--push"}, "create", "topic", "--data", `{"title":"Flag push"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if !gitPushed(t, m) || bareHead(t, bare) != headOf(t, a) {
		t.Fatalf("--push did not push: %s", s)
	}

	setGitPush(t, a, true)
	before := bareHead(t, bare)
	rc, m, s = runFlags(a, []string{"--no-push"}, "create", "topic", "--data", `{"title":"Held back"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if gitPushed(t, m) || bareHead(t, bare) != before {
		t.Fatalf("--no-push pushed: %s", s)
	}

	head := headOf(t, a)
	rc, m, s = runFlags(a, []string{"--push", "--no-push"}, "create", "topic", "--data", `{"title":"Both"}`)
	if rc == 0 {
		t.Fatalf("both flags accepted: %s", s)
	}
	if e, _ := m["error"].(map[string]any); e == nil || e["code"] != "BAD_INPUT" {
		t.Fatalf("error: %s", s)
	}
	if headOf(t, a) != head || bareHead(t, bare) != before {
		t.Fatal("both flags changed something")
	}
	if _, err := os.Stat(filepath.Join(a, "topics", "both.md")); err == nil {
		t.Fatal("topic written")
	}
}

func TestPushFailedRetried(t *testing.T) {
	bare, a, _ := syncPair(t)
	setGitPush(t, a, true)
	before := bareHead(t, bare)
	gitOut(t, a, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))

	rc, m, s := runCmd(a, "create", "topic", "--data", `{"title":"First"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	firstTopic := m
	if gitPushed(t, m) || !hasCode(m, "warnings", "PUSH_FAILED") {
		t.Fatalf("want PUSH_FAILED warning: %s", s)
	}
	first := headOf(t, a)
	if _, err := os.Stat(filepath.Join(a, "topics", "first.md")); err != nil {
		t.Fatal("commit not kept")
	}
	if st := gitOut(t, a, "status", "--porcelain"); st != "" {
		t.Fatalf("tree dirty: %s", st)
	}
	// add reports it in errors.
	rc, m, s = runCmd(a, "add", "Wind erodes soft sandstone into arches.", "--as", "article")
	if rc != 0 {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if gitPushed(t, m) || !hasCode(m, "errors", "PUSH_FAILED") {
		t.Fatalf("add errors lack PUSH_FAILED: %s", s)
	}
	if bareHead(t, bare) != before {
		t.Fatal("bare changed while broken")
	}

	// Fixed URL: the next pushing run sends every pending commit.
	gitOut(t, a, "remote", "set-url", "origin", bare)
	rc, m, s = runCmd(a, "create", "topic", "--data", `{"title":"Second"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if !gitPushed(t, m) || bareHead(t, bare) != headOf(t, a) {
		t.Fatalf("retry did not push: %s", s)
	}
	if out := gitOut(t, bare, "merge-base", "--is-ancestor", first, "HEAD"); out != "" {
		t.Fatalf("first commit missing: %s", out)
	}

	// A run without a commit pushes pending commits when ahead of upstream.
	firstID, _ := firstTopic["id"].(string)
	secondID, _ := m["id"].(string)
	rc, _, s = runCmd(a, "link", secondID, firstID)
	if rc != 0 {
		t.Fatalf("link rc=%d %s", rc, s)
	}
	gitOut(t, a, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	mustCreate(t, a, "topic", `{"title":"Third"}`)
	gitOut(t, a, "remote", "set-url", "origin", bare)
	head := headOf(t, a)
	if bareHead(t, bare) == head {
		t.Fatal("Third already pushed")
	}
	rc, m, s = runCmd(a, "link", secondID, firstID)
	if rc != 0 {
		t.Fatalf("link rc=%d %s", rc, s)
	}
	if m["changed"] != false || headOf(t, a) != head {
		t.Fatalf("second link made a commit: %s", s)
	}
	if !gitPushed(t, m) || bareHead(t, bare) != head {
		t.Fatalf("no-commit run did not push pending commits: %s", s)
	}
	// Nothing pending and no commit: nothing to push.
	rc, m, s = runCmd(a, "link", secondID, firstID)
	if rc != 0 || gitPushed(t, m) {
		t.Fatalf("idle run pushed: rc=%d %s", rc, s)
	}
}

func TestPushThenPull(t *testing.T) {
	bare, a, b := syncPair(t)
	setGitPush(t, a, true)
	rc, m, s := runCmd(a, "add", "Glaciers carve deep valleys over many thousand years of slow ice flow.", "--as", "article")
	if rc != 0 {
		t.Fatalf("A add rc=%d %s", rc, s)
	}
	if !gitPushed(t, m) || bareHead(t, bare) != headOf(t, a) {
		t.Fatalf("A did not push: %s", s)
	}
	atoms, _ := m["atoms"].([]any)
	if len(atoms) == 0 {
		t.Fatalf("A wrote no atoms: %s", s)
	}
	aID := strings.Trim(atoms[0].(map[string]any)["id"].(string), "[]")

	rc, m, s = runCmd(b, "create", "topic", "--data", `{"title":"From B"}`)
	if rc != 0 {
		t.Fatalf("B rc=%d %s", rc, s)
	}
	if n := gitPulled(t, m); n < 1 {
		t.Fatalf("B pulled=%v: %s", n, s)
	}
	if gitPushed(t, m) {
		t.Fatalf("B pushed with default config: %s", s)
	}
	if _, err := os.Stat(filepath.Join(b, "notes", aID+".md")); err != nil {
		t.Fatalf("A's note not in B: %v", err)
	}
	if n := strings.TrimSpace(gitOut(t, b, "rev-list", "--merges", "--count", "HEAD")); n != "0" {
		t.Fatalf("merge commits: %s", n)
	}
}
