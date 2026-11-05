package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPullPush(t *testing.T) {
	bare, a, b := syncPair(t)
	mustCreate(t, a, "topic", `{"title":"From A","body":"a"}`)
	gitOut(t, a, "push", "-q")
	aHead := headOf(t, a)
	if rc, _, s := runFlags(b, []string{"--no-pull"}, "create", "topic", "--data", `{"title":"From B","body":"b"}`); rc != 0 {
		t.Fatalf("B create rc=%d %s", rc, s)
	}
	before := treeSnapshot(t, b)
	rc, m, s := runCmd(b, "sync")
	if rc != 0 {
		t.Fatalf("sync rc=%d %s", rc, s)
	}
	if m["pulled"] != float64(1) || m["pushed"] != true {
		t.Fatalf("sync result: %s", s)
	}
	if bareHead(t, bare) != headOf(t, b) {
		t.Fatal("bare repo lacks B's head")
	}
	gitOut(t, bare, "merge-base", "--is-ancestor", aHead, "HEAD")
	if st := gitOut(t, b, "status", "--porcelain"); st != "" {
		t.Fatalf("status not clean: %s", st)
	}
	after := treeSnapshot(t, b)
	for p, c := range before {
		if after[p] != c {
			t.Fatalf("%s changed", p)
		}
	}
	pulled := filepath.Join(b, "topics", "from-a.md")
	for p := range after {
		if _, ok := before[p]; !ok && p != pulled {
			t.Fatalf("unexpected file %s", p)
		}
	}
	if _, ok := after[pulled]; !ok {
		t.Fatal("A's topic not pulled")
	}
}

func TestSyncNoRemote(t *testing.T) {
	isolateGitEnv(t)
	d := newFakeVault(t)
	head := headOf(t, d)
	rc, m, s := runCmd(d, "sync")
	if rc != 2 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	e, _ := m["error"].(map[string]any)
	if e == nil || e["code"] != "NOT_FOUND" || e["hint"] != "git remote add origin <url>" {
		t.Fatalf("error: %s", s)
	}
	if headOf(t, d) != head {
		t.Fatal("HEAD moved")
	}
}

func TestSyncFlags(t *testing.T) {
	bare, a, b := syncPair(t)
	// --no-pull: B's own commit is pushed, nothing pulled.
	if rc, _, s := runFlags(b, []string{"--no-pull"}, "create", "topic", "--data", `{"title":"From B"}`); rc != 0 {
		t.Fatalf("B create rc=%d %s", rc, s)
	}
	rc, m, s := runFlags(b, []string{"--no-pull"}, "sync")
	if rc != 0 || m["pulled"] != float64(0) || m["pushed"] != true {
		t.Fatalf("--no-pull sync rc=%d %s", rc, s)
	}
	if bareHead(t, bare) != headOf(t, b) {
		t.Fatal("B's commit not pushed")
	}
	// --no-push: A gets B's commit, nothing is pushed.
	if rc, _, s := runFlags(a, []string{"--no-pull"}, "create", "topic", "--data", `{"title":"From A"}`); rc != 0 {
		t.Fatalf("A create rc=%d %s", rc, s)
	}
	bh := bareHead(t, bare)
	rc, m, s = runFlags(a, []string{"--no-push"}, "sync")
	if rc != 0 || m["pulled"] != float64(1) || m["pushed"] != false {
		t.Fatalf("--no-push sync rc=%d %s", rc, s)
	}
	if bareHead(t, bare) != bh {
		t.Fatal("--no-push pushed")
	}
	if _, err := os.Stat(filepath.Join(a, "topics", "from-b.md")); err != nil {
		t.Fatal("B's topic not pulled into A")
	}
	// Both: nothing happens.
	head := headOf(t, a)
	rc, m, s = runFlags(a, []string{"--no-pull", "--no-push"}, "sync")
	if rc != 0 || m["pulled"] != float64(0) || m["pushed"] != false || headOf(t, a) != head {
		t.Fatalf("--no-pull --no-push sync rc=%d %s", rc, s)
	}
}

func TestDryRunBehind(t *testing.T) {
	_, a, b := syncPair(t)
	mustCreate(t, a, "topic", `{"title":"Ahead"}`)
	gitOut(t, a, "push", "-q")
	gitOut(t, b, "fetch", "-q")
	head := headOf(t, b)
	refs := gitOut(t, b, "for-each-ref")
	before := treeSnapshot(t, b)
	rc, m, s := runFlags(b, []string{"--dry-run"}, "create", "topic", "--data", `{"title":"Dry"}`)
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, s)
	}
	g, _ := m["git"].(map[string]any)
	if g == nil || g["behind"] != float64(1) || g["pulled"] != float64(0) || g["pushed"] != false {
		t.Fatalf("git: %s", s)
	}
	if !hasCode(m, "warnings", "BEHIND") {
		t.Fatalf("no BEHIND warning: %s", s)
	}
	if headOf(t, b) != head || gitOut(t, b, "for-each-ref") != refs {
		t.Fatal("refs changed")
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
	// add reports BEHIND in errors and git.behind in the report.
	rc, m, s = runFlags(b, []string{"--dry-run"}, "add", "Rivers move sediment downstream in floods.", "--as", "article")
	if rc != 0 || !hasCode(m, "errors", "BEHIND") {
		t.Fatalf("add rc=%d %s", rc, s)
	}
	if g, _ := m["git"].(map[string]any); g == nil || g["behind"] != float64(1) {
		t.Fatalf("add git: %s", s)
	}
	// collide too.
	rc, m, s = runFlags(b, []string{"--dry-run"}, "collide")
	if rc != 0 || !hasCode(m, "errors", "BEHIND") {
		t.Fatalf("collide rc=%d %s", rc, s)
	}
	if headOf(t, b) != head || gitOut(t, b, "status", "--porcelain") != "" {
		t.Fatal("dry runs changed the repo")
	}
}

func TestSearchSync(t *testing.T) {
	_, a, b := syncPair(t)
	src := mustCreate(t, a, "source", `{"medium":"article","title":"Ice","author":"A"}`)["id"].(string)
	nid := mustCreate(t, a, "note", `{"claim":"Glaciers carve deep valleys","evidence":"glaciers carve valleys","stance":"asserts","locator":"line 1","source":"`+src+`"}`)["id"].(string)
	gitOut(t, a, "push", "-q")
	head := headOf(t, b)
	rc, m, s := runCmd(b, "search", "glaciers")
	if rc != 0 {
		t.Fatalf("search rc=%d %s", rc, s)
	}
	if headOf(t, b) != head {
		t.Fatal("search pulled")
	}
	if strings.Contains(s, nid) {
		t.Fatalf("search found A's note before the pull: %s", s)
	}
	if _, ok := m["pulled"]; ok {
		t.Fatalf("pulled without --sync: %s", s)
	}
	rc, m, s = runFlags(b, []string{"--sync"}, "search", "glaciers")
	if rc != 0 {
		t.Fatalf("search --sync rc=%d %s", rc, s)
	}
	if headOf(t, b) == head {
		t.Fatal("search --sync did not pull")
	}
	if m["pulled"] != float64(2) {
		t.Fatalf("pulled: %s", s)
	}
	hits, _ := m["hits"].([]any)
	found := false
	for _, h := range hits {
		if hm, ok := h.(map[string]any); ok && hm["id"] == nid {
			found = true
		}
	}
	if !found {
		t.Fatalf("hits lack %s: %s", nid, s)
	}
	// --sync is ignored by a write command.
	if rc, _, s := runFlags(b, []string{"--sync", "--no-pull"}, "create", "topic", "--data", `{"title":"Local"}`); rc != 0 {
		t.Fatalf("create --sync rc=%d %s", rc, s)
	}
}
