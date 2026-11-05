package cli

import "testing"

func listIDs(t *testing.T, d string, args ...string) []string {
	t.Helper()
	rc, m, s := runCmd(d, append([]string{"list"}, args...)...)
	if rc != 0 {
		t.Fatalf("list %v rc=%d %s", args, rc, s)
	}
	var ids []string
	for _, it := range m["items"].([]any) {
		ids = append(ids, it.(map[string]any)["id"].(string))
	}
	if int(m["count"].(float64)) != len(ids) {
		t.Fatalf("count mismatch %s", s)
	}
	return ids
}

type seeded struct{ d, poem, paper, nPoem, nPaper, hunch string }

func seedList(t *testing.T) seeded {
	d := newVault(t)
	id := func(m map[string]any) string { return m["id"].(string) }
	var s seeded
	s.d = d
	s.poem = id(mustCreate(t, d, "source", `{"medium":"poem","title":"Trees","author":"A"}`))
	s.paper = id(mustCreate(t, d, "source", `{"medium":"paper","title":"Paper","author":"B"}`))
	s.nPoem = id(mustCreate(t, d, "note", `{"claim":"n poem","evidence":"e","stance":"crafts","locator":"line 1","source":"`+s.poem+`","tags":["rhythm"]}`))
	s.nPaper = id(mustCreate(t, d, "note", `{"claim":"n paper","evidence":"e","stance":"asserts","locator":"line 1","source":"`+s.paper+`"}`))
	s.hunch = id(mustCreate(t, d, "hunch", `{"question":"Why?"}`))
	mustCreate(t, d, "topic", `{"title":"Rhythm"}`)
	return s
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCRUDList(t *testing.T) {
	s := seedList(t)
	d := s.d
	ids := listIDs(t, d)
	if len(ids) != 6 {
		t.Fatalf("%v", ids)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("not sorted %v", ids)
		}
	}
}

func TestCRUDListFilters(t *testing.T) {
	s := seedList(t)
	d := s.d
	if got := listIDs(t, d, "--type", "source"); len(got) != 2 {
		t.Fatal(got)
	}
	if got := listIDs(t, d, "--medium", "poem"); len(got) != 2 {
		t.Fatalf("medium poem: %v", got)
	}
	if got := listIDs(t, d, "--stance", "asserts"); !eq(got, []string{s.nPaper}) {
		t.Fatalf("stance: %v", got)
	}
	if got := listIDs(t, d, "--tag", "rhythm"); !eq(got, []string{s.nPoem}) {
		t.Fatalf("tag: %v", got)
	}
	if got := listIDs(t, d, "--state", "open"); !eq(got, []string{s.hunch}) {
		t.Fatalf("state: %v", got)
	}
	if got := listIDs(t, d, "--hunch", s.hunch); len(got) != 0 {
		t.Fatalf("hunch: %v", got)
	}
	if got := listIDs(t, d, "--type", "note", "--medium", "poem"); !eq(got, []string{s.nPoem}) {
		t.Fatalf("combined: %v", got)
	}
	if got := listIDs(t, d, "--limit", "2"); len(got) != 2 {
		t.Fatal(got)
	}
}

func TestCRUDListBadFilter(t *testing.T) {
	s := seedList(t)
	d := s.d
	for _, a := range [][]string{{"--stance", "shouts"}, {"--type", "x"}, {"--medium", "x"}, {"--state", "x"}} {
		if rc, _, s := runCmd(d, append([]string{"list"}, a...)...); rc != 2 {
			t.Fatalf("%v rc=%d %s", a, rc, s)
		}
	}
}
