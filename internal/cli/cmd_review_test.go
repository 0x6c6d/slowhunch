package cli

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func reviewTree(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, rel+" "+string(sha256.New().Sum(b)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	if _, err := exec.LookPath("git"); err == nil {
		out, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").CombinedOutput()
		lines = append(lines, "HEAD "+string(out))
	}
	return strings.Join(lines, "\n")
}

func strList(t *testing.T, m map[string]any, k string) []string {
	t.Helper()
	a, ok := m[k].([]any)
	if !ok {
		t.Fatalf("%s is not an array: %v", k, m)
	}
	out := []string{}
	for _, x := range a {
		out = append(out, x.(string))
	}
	return out
}

func TestReviewCommand(t *testing.T) {
	d := newVault(t)
	fresh := addHunch(t, d, "Fresh hunch?")
	old := "---\nid: h-old\ntype: hunch\nquestion: q\nstate: open\ncreated: 2020-01-01\n---\n"
	if err := os.WriteFile(filepath.Join(d, "hunches", "h-old.md"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	before := reviewTree(t, d)
	rc, m, s := runCmd(d, "review")
	if rc != 0 {
		t.Fatal(s)
	}
	if got := strList(t, m, "stale_hunches"); !reflect.DeepEqual(got, []string{"h-old"}) {
		t.Fatalf("stale %v (fresh=%s) %s", got, fresh, s)
	}
	if got := strList(t, m, "orphans"); len(got) != 0 {
		t.Fatalf("orphans %v", got)
	}
	if got := strList(t, m, "unplaced"); len(got) != 0 {
		t.Fatalf("unplaced %v", got)
	}
	if reviewTree(t, d) != before {
		t.Fatal("review changed the vault")
	}
	for _, a := range []string{"0", "-3"} {
		rc, m, s := runCmd(d, "review", "--stale-days", a)
		if e, _ := m["error"].(map[string]any); rc == 0 || e["code"] != "BAD_INPUT" {
			t.Fatalf("stale-days %s: rc=%d %s", a, rc, s)
		}
	}
	if reviewTree(t, d) != before {
		t.Fatal("failed review changed the vault")
	}
}
