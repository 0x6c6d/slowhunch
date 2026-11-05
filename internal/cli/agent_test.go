package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// agentRun runs the CLI in-process and decodes stdout as JSON.
func agentRun(t *testing.T, d string, args ...string) (int, map[string]any) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := Run(append(append([]string{}, args...), "--vault", d, "--json"), strings.NewReader(""), &out, &errb)
	m := map[string]any{}
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("%v: stdout not JSON: %q", args, out.String())
	}
	return rc, m
}

func revCount(t *testing.T, d string) int {
	t.Helper()
	var n int
	for _, c := range strings.TrimSpace(gitOut(t, d, "rev-list", "--count", "HEAD")) {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestAgentSequence(t *testing.T) {
	d := newVault(t)
	seen := map[string]bool{}
	step := func(commit bool, args ...string) map[string]any {
		t.Helper()
		before := revCount(t, d)
		rc, m := agentRun(t, d, args...)
		if rc != 0 || m["ok"] != true {
			t.Fatalf("%v: rc=%d %v", args, rc, m)
		}
		want := before
		if commit {
			want++
		}
		if n := revCount(t, d); n != want {
			t.Fatalf("%v: commits %d -> %d, want %d", args, before, n, want)
		}
		if id, ok := m["id"].(string); ok && (args[0] == "create" || args[1] == "add") {
			if seen[id] {
				t.Fatalf("%v: id %s reused", args, id)
			}
			seen[id] = true
		}
		return m
	}
	src := step(true, "create", "source", "--data", `{"medium":"article","title":"Sleep","author":"A. Writer"}`)["id"].(string)
	note := step(true, "create", "note", "--data",
		`{"claim":"Sleep consolidates memory","evidence":"consolidation happens overnight","stance":"asserts","locator":"line 1","source":"`+src+`"}`)["id"].(string)
	topic := step(true, "create", "topic", "--data", `{"title":"Memory"}`)["id"].(string)
	step(true, "link", note, topic)
	h := step(true, "hunch", "add", "Does sleep help recall?")["id"].(string)
	step(true, "hunch", "evidence", h, "--note", note, "--relation", "supports")
	step(true, "delete", topic)
	if _, err := os.Stat(filepath.Join(d, "archive", topic+".md")); err != nil {
		t.Fatalf("topic not archived: %v", err)
	}
	step(false, "lint")
	step(false, "backlinks", note)
}

func TestSchemaAllCommands(t *testing.T) {
	d := newFakeVault(t)
	src := mustCreate(t, d, "source", `{"medium":"article","title":"Sleep","author":"A"}`)["id"].(string)
	note := mustCreate(t, d, "note", `{"claim":"c","evidence":"e","stance":"asserts","locator":"line 1","source":"`+src+`"}`)
	nid := note["id"].(string)
	t1 := mustCreate(t, d, "topic", `{"title":"One"}`)
	t2 := mustCreate(t, d, "topic", `{"title":"Two"}`)["id"].(string)
	t3 := mustCreate(t, d, "topic", `{"title":"Three"}`)["id"].(string)
	t1id := t1["id"].(string)
	if rc, _ := agentRun(t, d, "link", t1id, t3); rc != 0 {
		t.Fatal("seed link")
	}
	h := mustCreate(t, d, "hunch", `{"question":"Why?"}`)["id"].(string)
	fresh := filepath.Join(t.TempDir(), "fresh")

	valid := map[string][]string{
		"add":            {"Sleep matters for memory. Studies show it.", "--as", "article"},
		"backlinks":      {nid},
		"create":         {"topic", "--data", `{"title":"Four"}`},
		"delete":         {t2},
		"get":            {nid},
		"graph":          {nid},
		"hunch add":      {"A question?"},
		"hunch evidence": {h, "--note", nid, "--relation", "supports"},
		"hunch state":    {h, "tested"},
		"hunch due":      nil,
		"init":           {fresh},
		"link":           {nid, t3},
		"unlink":         {t1id, t3},
		"lint":           nil,
		"list":           nil,
		"search":         {"sleep"},
		"skill":          nil,
		"source facts":   {src},
		"stats":          nil,
		"update":         {t1id, "--rev", t1["rev"].(string), "--data", `{"tags":["x"]}`},
		"verify":         nil,
	}
	names := make([]string, 0, len(commands))
	for n := range commands {
		if strings.HasPrefix(n, "t ") { // fixtures registered by cli_test.go
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	check := func(name string, args []string) {
		t.Helper()
		full := append(strings.Fields(name), args...)
		rc, m := agentRun(t, d, full...)
		if m["schema"] != float64(1) {
			t.Fatalf("%v: schema %v", full, m["schema"])
		}
		if rc == 0 {
			return
		}
		e, ok := m["error"].(map[string]any)
		if !ok {
			t.Fatalf("%v: rc=%d without error object: %v", full, rc, m)
		}
		for _, k := range []string{"code", "message"} {
			if s, _ := e[k].(string); s == "" {
				t.Fatalf("%v: empty %s: %v", full, k, e)
			}
		}
		if _, ok := e["hint"].(string); !ok {
			t.Fatalf("%v: hint missing: %v", full, e)
		}
	}
	for _, n := range names {
		args, ok := valid[n]
		if !ok {
			t.Fatalf("command %q has no schema case", n)
		}
		check(n, args)
		// commands needing arguments also get the empty-argument error shape
		check(n, nil)
	}
}

func TestExitCodesAll(t *testing.T) {
	d := newVault(t)
	tp := mustCreate(t, d, "topic", `{"title":"Trees"}`)
	old := tp["rev"].(string)

	expect := func(want int, args ...string) map[string]any {
		t.Helper()
		rc, m := agentRun(t, d, args...)
		if rc != want || m["schema"] != float64(1) {
			t.Fatalf("%v: rc=%d want %d: %v", args, rc, want, m)
		}
		return m
	}
	expect(0, "list")
	expect(2, "no-such-command")
	expect(2, "update", "trees", "--data", `{"tags":["a"]}`)
	expect(0, "update", "trees", "--rev", old, "--data", `{"tags":["a"]}`)
	m := expect(4, "update", "trees", "--rev", old, "--data", `{"tags":["b"]}`)
	if e, _ := m["error"].(map[string]any); e == nil || e["code"] != "CONFLICT_REV" {
		t.Fatalf("want CONFLICT_REV: %v", m)
	}

	// 3: lint failure (crafts note carrying confidence)
	d3 := newVault(t)
	for id, med := range map[string]string{"poem": "poem"} {
		if err := os.WriteFile(filepath.Join(d3, "sources", id+".md"), []byte("---\nid: "+id+"\ntype: source\nmedium: "+med+"\ntitle: t\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bad := lintNoteFile("v-conf", "2026-01-01", "stance: crafts\nsource: poem\nevidence: e\nlocator: l\nconfidence: \"0.5\"\n")
	if err := os.WriteFile(filepath.Join(d3, "notes", "v-conf.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, m := agentRun(t, d3, "lint"); rc != 3 || m["schema"] != float64(1) {
		t.Fatalf("lint rc=%d %v", rc, m)
	}

	// 1: invalid config.json is fatal
	if err := os.WriteFile(filepath.Join(d, ".slowhunch", "config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = expect(1, "list")
	if e, _ := m["error"].(map[string]any); e == nil || e["code"] == "" {
		t.Fatalf("want error object: %v", m)
	}
}
