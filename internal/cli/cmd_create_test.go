package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newVault(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "v")
	if rc, _, e := runInit(t, d); rc != 0 {
		t.Fatal(e)
	}
	return d
}

func runCmd(d string, args ...string) (int, map[string]any, string) {
	var out, errb bytes.Buffer
	rc := Run(append(args, "--vault", d, "--json"), strings.NewReader(""), &out, &errb)
	m := map[string]any{}
	_ = json.Unmarshal(out.Bytes(), &m)
	return rc, m, out.String() + errb.String()
}

func mustCreate(t *testing.T, d, typ, data string) map[string]any {
	t.Helper()
	rc, m, s := runCmd(d, "create", typ, "--data", data)
	if rc != 0 {
		t.Fatalf("create %s rc=%d %s", typ, rc, s)
	}
	return m
}

func TestCRUDCreateGet(t *testing.T) {
	d := newVault(t)
	src := mustCreate(t, d, "source", `{"medium":"poem","title":"Trees","author":"Joyce Kilmer"}`)
	sid := src["id"].(string)
	note := mustCreate(t, d, "note", `{"claim":"c one","evidence":"line a\nline b","stance":"crafts","locator":"line 1","source":"`+sid+`"}`)
	ids := []string{sid, note["id"].(string),
		mustCreate(t, d, "hunch", `{"question":"Why?"}`)["id"].(string),
		mustCreate(t, d, "proposal", `{"idea":"Try it"}`)["id"].(string),
		mustCreate(t, d, "topic", `{"title":"Rhythm"}`)["id"].(string)}
	for _, id := range ids {
		rc, m, s := runCmd(d, "get", id)
		if rc != 0 || m["id"] != id || m["fields"] == nil {
			t.Fatalf("get %s: %s", id, s)
		}
	}
	b, err := os.ReadFile(filepath.Join(d, "notes", note["id"].(string)+".md"))
	if err != nil || !strings.HasSuffix(string(b), "c one\n\n> line a\n> line b\n") {
		t.Fatalf("%v %q", err, b)
	}
	log := gitOut(t, d, "log", "--format=%s")
	if n := strings.Count(log, "slowhunch: create "); n != 5 {
		t.Fatalf("want 5 create commits: %s", log)
	}
}

func TestCRUDCreateCollision(t *testing.T) {
	d := newVault(t)
	mustCreate(t, d, "hunch", `{"question":"Same?"}`)
	m := mustCreate(t, d, "hunch", `{"question":"Same?"}`)
	if !strings.HasSuffix(m["id"].(string), "-2") {
		t.Fatal(m["id"])
	}
}

func TestCRUDCreateInvalid(t *testing.T) {
	d := newVault(t)
	before := gitOut(t, d, "log", "--format=%H")
	cases := [][]string{
		{"note", `{"claim":"c","evidence":"e","stance":"bogus","locator":"l"}`},
		{"hunch", `{"question":"q","nope":"x"}`},
		{"note", `{"claim":"c","evidence":"e","stance":"crafts","locator":"l","confidence":"0.5"}`},
		{"hunch", `{bad json`},
	}
	for _, c := range cases {
		if rc, _, s := runCmd(d, "create", c[0], "--data", c[1]); rc != 2 {
			t.Fatalf("%v rc=%d %s", c, rc, s)
		}
	}
	for _, f := range []string{"notes", "hunches"} {
		if es, _ := os.ReadDir(filepath.Join(d, f)); len(es) != 0 {
			t.Fatalf("%s not empty", f)
		}
	}
	if gitOut(t, d, "log", "--format=%H") != before {
		t.Fatal("commit made")
	}
}

func TestCRUDGetMissing(t *testing.T) {
	d := newVault(t)
	rc, _, s := runCmd(d, "get", "nope")
	if rc != 2 || !strings.Contains(s, "NOT_FOUND") {
		t.Fatalf("rc=%d %s", rc, s)
	}
}
