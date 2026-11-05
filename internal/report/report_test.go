package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func TestReportKeyOrder(t *testing.T) {
	b, err := json.Marshal(New("r1"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("null in %s", b)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.Token()
	var keys []string
	for dec.More() {
		tok, _ := dec.Token()
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"run_id", "source", "medium", "medium_inferred", "contradictions", "hunches_moved", "atoms", "corroborations", "echoes", "wildcards", "collisions", "counts", "errors", "cost", "git"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v", keys)
	}
}

func TestReportIDs(t *testing.T) {
	r := New("r1")
	r.Atoms = append(r.Atoms, AtomRef{ID: Link("b"), Target: Link("a")})
	r.Echoes = append(r.Echoes, Echo{Atom: Link("b"), Echoes: Link("c")})
	got := r.IDs()
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("ids %v", got)
	}
	if len(New("x").IDs()) != 0 {
		t.Fatal("empty report has ids")
	}
}

func tempVault(t *testing.T) *vault.Vault {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReportSaveLoad(t *testing.T) {
	v := tempVault(t)
	if rs, err := LoadAll(v); err != nil || len(rs) != 0 {
		t.Fatalf("empty: %v %v", rs, err)
	}
	b, a := New("run-b"), New("run-a")
	a.AddCost("extract", Usage{1, 2})
	b.AddCost("judge", Usage{3, 4})
	for _, r := range []*Report{b, a} {
		if err := r.Save(v); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := LoadAll(v)
	if err != nil || len(rs) != 2 {
		t.Fatalf("%v %v", rs, err)
	}
	if rs[0].RunID != "run-a" || rs[0].Cost["extract"] != (Usage{1, 2}) || rs[1].Cost["judge"] != (Usage{3, 4}) {
		t.Fatalf("loaded %+v %+v", rs[0], rs[1])
	}
	raw, _ := v.ReadFile(".slowhunch/reports/run-a.json")
	if !strings.HasPrefix(string(raw), "{\n  \"run_id\"") || !strings.HasSuffix(string(raw), "}\n") {
		t.Fatalf("format %q", raw)
	}
}

func TestReportAddCost(t *testing.T) {
	r := New("r")
	r.AddCost("x", Usage{1, 2})
	r.AddCost("x", Usage{10, 20})
	r.AddCost("y", Usage{5, 5})
	if r.Cost["x"] != (Usage{11, 22}) || r.Cost["y"] != (Usage{5, 5}) {
		t.Fatalf("%v", r.Cost)
	}
}
