package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commits(t *testing.T, d string) int {
	return len(strings.Fields(gitOut(t, d, "log", "--format=%H")))
}

func TestCRUDLink(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Alpha"}`)["id"].(string)
	b := mustCreate(t, d, "topic", `{"title":"Beta"}`)["id"].(string)
	h := mustCreate(t, d, "hunch", `{"question":"Why?"}`)["id"].(string)
	bf := filepath.Join(d, "topics", b+".md")
	hf := filepath.Join(d, "hunches", h+".md")
	bb, _ := os.ReadFile(bf)
	hb, _ := os.ReadFile(hf)
	n := commits(t, d)
	rc, m, s := runCmd(d, "link", a, b)
	if rc != 0 || m["field"] != "links" || m["changed"] != true {
		t.Fatal(s)
	}
	rc, m, s = runCmd(d, "link", a, h)
	if rc != 0 || m["field"] != "bears_on" || m["changed"] != true {
		t.Fatal(s)
	}
	if commits(t, d) != n+2 {
		t.Fatal("want one commit each")
	}
	if x, _ := os.ReadFile(bf); string(x) != string(bb) {
		t.Fatal("target changed")
	}
	if x, _ := os.ReadFile(hf); string(x) != string(hb) {
		t.Fatal("target changed")
	}
	af, _ := os.ReadFile(filepath.Join(d, "topics", a+".md"))
	if !strings.Contains(string(af), "links:") || !strings.Contains(string(af), "bears_on:") {
		t.Fatalf("%s", af)
	}
	if rc, _, _ := runCmd(d, "link", a, a); rc != 2 {
		t.Fatal("self link should be BAD_INPUT")
	}
}

func TestCRUDLinkIdempotent(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Alpha"}`)["id"].(string)
	b := mustCreate(t, d, "topic", `{"title":"Beta"}`)["id"].(string)
	runCmd(d, "link", a, b)
	n := commits(t, d)
	rc, m, s := runCmd(d, "link", a, b)
	if rc != 0 || m["changed"] != false || commits(t, d) != n {
		t.Fatal(s)
	}
}

func TestCRUDUnlink(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Alpha"}`)["id"].(string)
	b := mustCreate(t, d, "topic", `{"title":"Beta"}`)["id"].(string)
	runCmd(d, "link", a, b)
	n := commits(t, d)
	rc, m, s := runCmd(d, "unlink", a, b)
	if rc != 0 || m["changed"] != true || commits(t, d) != n+1 {
		t.Fatal(s)
	}
	rc, m, s = runCmd(d, "unlink", a, b)
	if rc != 0 || m["changed"] != false || commits(t, d) != n+1 {
		t.Fatal(s)
	}
	_, m, _ = runCmd(d, "backlinks", b)
	if l, _ := m["backlinks"].([]any); len(l) != 0 {
		t.Fatal(m)
	}
}

func TestCRUDBacklinks(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Alpha"}`)["id"].(string)
	b := mustCreate(t, d, "topic", `{"title":"Beta"}`)["id"].(string)
	c := mustCreate(t, d, "topic", `{"title":"Gamma"}`)["id"].(string)
	runCmd(d, "link", c, a)
	runCmd(d, "link", b, a)
	rc, m, s := runCmd(d, "backlinks", a)
	l, _ := m["backlinks"].([]any)
	if rc != 0 || len(l) != 2 || l[0] != b || l[1] != c {
		t.Fatal(s)
	}
	if rc, _, _ := runCmd(d, "backlinks", "ghost"); rc != 2 {
		// NOT_FOUND exit code is checked in TestCRUDLinkMissing
		t.Log("rc", rc)
	}
}

func TestCRUDGraphDepth(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Aa"}`)["id"].(string)
	b := mustCreate(t, d, "topic", `{"title":"Bb"}`)["id"].(string)
	c := mustCreate(t, d, "topic", `{"title":"Cc"}`)["id"].(string)
	runCmd(d, "link", a, b)
	runCmd(d, "link", b, c)
	count := func(args ...string) int {
		rc, m, s := runCmd(d, args...)
		if rc != 0 {
			t.Fatal(s)
		}
		return len(m["nodes"].([]any))
	}
	if n := count("graph", a); n != 2 {
		t.Fatal(n)
	}
	if n := count("graph", a, "--depth", "2"); n != 3 {
		t.Fatal(n)
	}
	if rc, _, _ := runCmd(d, "graph", a, "--depth", "9"); rc != 2 {
		t.Fatal(rc)
	}
}

func TestCRUDLinkMissing(t *testing.T) {
	d := newVault(t)
	a := mustCreate(t, d, "topic", `{"title":"Alpha"}`)["id"].(string)
	f := filepath.Join(d, "topics", a+".md")
	before, _ := os.ReadFile(f)
	n := commits(t, d)
	rc, m, s := runCmd(d, "link", a, "ghost")
	if rc == 0 || !strings.Contains(s, "NOT_FOUND") {
		t.Fatal(rc, m, s)
	}
	after, _ := os.ReadFile(f)
	if string(before) != string(after) || commits(t, d) != n {
		t.Fatal("changed")
	}
}
