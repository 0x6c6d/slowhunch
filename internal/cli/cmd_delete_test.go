package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCRUDDeleteSoft(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	before := commitCount(t, d)
	rc, r, s := runCmd(d, "delete", "trees")
	if rc != 0 || r["archived"] != "archive/trees.md" || r["id"] != "trees" {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if _, err := os.Stat(filepath.Join(d, "archive", "trees.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d, "topics", "trees.md")); !os.IsNotExist(err) {
		t.Fatalf("still in topics: %v", err)
	}
	if rc, _, s := runCmd(d, "get", "trees"); rc != 2 || !strings.Contains(s, "NOT_FOUND") {
		t.Fatalf("get after delete rc=%d %s", rc, s)
	}
	if n := commitCount(t, d); n != before+1 {
		t.Fatalf("want one new commit, got %d", n-before)
	}
	names := gitOut(t, d, "show", "--name-status", "--no-renames", "--format=%s", "HEAD")
	if !strings.Contains(names, "D\ttopics/trees.md") || !strings.Contains(names, "A\tarchive/trees.md") ||
		!strings.Contains(names, "slowhunch: delete trees (archived: 1)") {
		t.Fatal(names)
	}
	if st := gitOut(t, d, "status", "--porcelain", "--", "topics", "archive"); st != "" {
		t.Fatal(st)
	}
	// Second soft delete of a re-created id collides with the archive.
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	if rc, _, s := runCmd(d, "delete", "trees"); rc != 4 || !strings.Contains(s, "EXISTS") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if _, err := os.Stat(filepath.Join(d, "topics", "trees.md")); err != nil {
		t.Fatal("moved despite EXISTS")
	}
}

func TestCRUDDeleteHardNeedsYes(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	orig := readVaultFile(t, d, "topics/trees.md")
	before := commitCount(t, d)
	rc, _, s := runCmd(d, "delete", "trees", "--hard")
	if rc != 2 || !strings.Contains(s, "hard delete needs --yes") {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if readVaultFile(t, d, "topics/trees.md") != orig {
		t.Fatal("file changed")
	}
	if commitCount(t, d) != before {
		t.Fatal("commit made")
	}
	// --yes alone is a soft delete.
	if rc, r, s := runCmd(d, "delete", "trees", "--yes"); rc != 0 || r["archived"] != "archive/trees.md" {
		t.Fatalf("rc=%d %s", rc, s)
	}
}

func TestCRUDDeleteHard(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	mustCreate(t, d, "topic", `{"title":"Other","body":"see [[trees]]"}`)
	before := commitCount(t, d)
	rc, r, s := runCmd(d, "delete", "trees", "--hard", "--yes")
	if rc != 0 || r["removed"] != true {
		t.Fatalf("rc=%d %s", rc, s)
	}
	if _, err := os.Stat(filepath.Join(d, "topics", "trees.md")); !os.IsNotExist(err) {
		t.Fatal("file still present")
	}
	if _, err := os.Stat(filepath.Join(d, "archive", "trees.md")); !os.IsNotExist(err) {
		t.Fatal("archived on hard delete")
	}
	if _, err := os.Stat(filepath.Join(d, "topics", "other.md")); err != nil {
		t.Fatal("linking file touched")
	}
	if n := commitCount(t, d); n != before+1 {
		t.Fatalf("want one new commit, got %d", n-before)
	}
	names := gitOut(t, d, "show", "--name-status", "--format=%s", "HEAD")
	if !strings.Contains(names, "D\ttopics/trees.md") || !strings.Contains(names, "(removed: 1)") {
		t.Fatal(names)
	}
	if st := gitOut(t, d, "status", "--porcelain", "--", "topics"); st != "" {
		t.Fatal(st)
	}
}

func TestCRUDDeleteMissing(t *testing.T) {
	d := newVault(t)
	for _, args := range [][]string{{"delete", "nope"}, {"delete", "nope", "--hard", "--yes"}} {
		if rc, _, s := runCmd(d, args...); rc != 2 || !strings.Contains(s, "NOT_FOUND") {
			t.Fatalf("%v rc=%d %s", args, rc, s)
		}
	}
}

func TestCRUDDeleteDryRun(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "topic", `{"title":"Trees"}`)
	orig := readVaultFile(t, d, "topics/trees.md")
	before := commitCount(t, d)
	for _, args := range [][]string{{"--dry-run", "delete", "trees"}, {"--dry-run", "delete", "trees", "--hard", "--yes"}} {
		if rc, _, s := runCmd(d, args...); rc != 0 {
			t.Fatalf("%v rc=%d %s", args, rc, s)
		}
		if readVaultFile(t, d, "topics/trees.md") != orig {
			t.Fatal("file changed")
		}
		if _, err := os.Stat(filepath.Join(d, "archive", "trees.md")); !os.IsNotExist(err) {
			t.Fatal("archived in dry run")
		}
		if commitCount(t, d) != before {
			t.Fatal("commit made")
		}
	}
}
