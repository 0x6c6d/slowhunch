package pipeline

import (
	"context"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func TestCapsRunsPerDay(t *testing.T) {
	v, r := addVault(t)
	f := addFake()
	d := addDeps(v, r, f)
	d.Config.Caps.ByMedium = map[string]vault.MediumCaps{"article": {RunsPerDay: 1}}
	ctx := context.Background()
	day1 := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return day1 }

	if _, err := Add(ctx, d, Options{Arg: addArticle, Medium: "article"}); err != nil {
		t.Fatal(err)
	}
	st, _ := v.LoadState()
	if st.RunsByDay["2026-11-05"]["article"] != 1 {
		t.Fatalf("state %+v", st)
	}

	// Second article run the same day: refused, nothing sent, nothing written.
	calls, before := len(f.Calls), treeHash(t, v.Root)
	_, err := Add(ctx, d, Options{Arg: addArticle2, Medium: "article"})
	if errs.CodeOf(err) != errs.CapReached || errs.ExitCode(err) != 2 {
		t.Fatalf("err %v", err)
	}
	if len(f.Calls) != calls || treeHash(t, v.Root) != before {
		t.Fatal("capped run sent or wrote something")
	}

	// A poem run is unaffected.
	if _, err := Add(ctx, d, Options{Arg: addArticle2, Medium: "poem", Title: "Rain", Author: "Ada"}); err != nil {
		t.Fatal(err)
	}

	// A batch with an article and a poem is refused as a whole.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("Rain fell on the hill all night.\n\nThe river rose by morning."), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("Snow covered the road at dawn.\n\nNobody drove to town."), 0o644)
	bf := filepath.Join(dir, "batch.txt")
	os.WriteFile(bf, []byte("a.txt\nb.txt\n"), 0o644)
	calls, before = len(f.Calls), treeHash(t, v.Root)
	_, err = AddBatch(ctx, d, bf, Options{Medium: "article"})
	if errs.CodeOf(err) != errs.CapReached || len(f.Calls) != calls || treeHash(t, v.Root) != before {
		t.Fatalf("batch err %v", err)
	}

	// Dry run does not count; next day succeeds and old dates are pruned.
	d.Now = func() time.Time { return day1.AddDate(0, 0, 1) }
	if _, err := Add(ctx, d, Options{Arg: addArticle2, Medium: "article", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if st, _ = v.LoadState(); st.RunsByDay["2026-11-06"]["article"] != 0 {
		t.Fatalf("dry run counted: %+v", st)
	}
	if _, err := Add(ctx, d, Options{Arg: addArticle2, Medium: "article"}); err != nil {
		t.Fatal(err)
	}
	st, _ = v.LoadState()
	if len(st.RunsByDay) != 1 || st.RunsByDay["2026-11-06"]["article"] != 1 {
		t.Fatalf("state %+v", st.RunsByDay)
	}
}

func TestCapsTokensPerRun(t *testing.T) {
	long := func(topic string) string {
		return topic + " is the first sentence of this entry.\n\n" + strings.Repeat(topic+" appears again in a filler line of the article.\n", 25)
	}
	setup := func(t *testing.T, limit int) (*vault.Vault, *fake.Fake, Deps, string) {
		v, r := addVault(t)
		f := addFake()
		inner := f.Handler
		f.Handler = func(req provider.Request) (provider.Response, error) {
			resp, err := inner(req)
			resp.TokensIn, resp.TokensOut = 6, 4 // every call reports 10 tokens
			return resp, err
		}
		d := addDeps(v, r, f)
		d.Config.Caps.ByMedium = map[string]vault.MediumCaps{"article": {TokensPerRun: limit}}
		dir := t.TempDir()
		var lines []string
		for i, topic := range []string{"Zebra", "Quartz", "Harbor", "Violin"} {
			name := "a" + string(rune('1'+i)) + ".txt"
			os.WriteFile(filepath.Join(dir, name), []byte(long(topic)), 0o644)
			lines = append(lines, name)
		}
		os.WriteFile(filepath.Join(dir, "p.txt"), []byte("Rain fell on the hill all night.\n\nThe river rose by morning."), 0o644)
		lines = append(lines, "p.txt")
		bf := filepath.Join(dir, "batch.txt")
		os.WriteFile(bf, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		return v, f, d, bf
	}
	heads := func(t *testing.T, v *vault.Vault) int {
		out, err := exec.Command("git", "-C", v.Root, "rev-list", "--count", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		return n
	}

	// Limit 50: entries 1 and 2 (entry 2 also places) finish; 3 and 4 are refused.
	v, f, d, bf := setup(t, 50)
	before := heads(t, v)
	res, err := AddBatch(context.Background(), d, bf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts["written"] != 3 || res.Counts["failed"] != 2 || len(res.Errors) != 2 {
		t.Fatalf("counts %+v errors %+v", res.Counts, res.Errors)
	}
	for k, line := range []string{"line 3: a3.txt", "line 4: a4.txt"} {
		if e := res.Errors[k]; e.Item != line || e.Code != errs.CapReached {
			t.Fatalf("error %d: %+v", k, e)
		}
	}
	for _, c := range f.Calls {
		if strings.Contains(c.User, "Harbor") || strings.Contains(c.User, "Violin") {
			t.Fatalf("provider called for a refused entry: %s", c.Role)
		}
	}
	if got := heads(t, v); got != before+1 {
		t.Fatalf("commits %d -> %d", before, got)
	}
	if n := len(res.Items); n != 5 || res.Items[2].Source != "" || res.Items[3].Source != "" {
		t.Fatalf("items %d", n)
	}

	// Limit 0 processes everything.
	v, _, d, bf = setup(t, 0)
	res, err = AddBatch(context.Background(), d, bf, Options{})
	if err != nil || res.Counts["written"] != 5 || len(res.Errors) != 0 {
		t.Fatalf("unlimited: %v %+v", err, res)
	}

	// A single add refused before its first call: not possible with a fresh meter,
	// so a limit of 1 still lets the first call finish and fails the entry.
	v, f, d, _ = setup(t, 1)
	tree := treeHash(t, v.Root)
	rep, err := Add(context.Background(), d, Options{Arg: long("Zebra"), Medium: "article"})
	if err != nil || rep.Source != "" || len(rep.Errors) != 1 || rep.Errors[0].Code != errs.CapReached {
		t.Fatalf("single: %v %+v", err, rep)
	}
	if len(f.Calls) != 1 || treeHash(t, v.Root) != tree {
		t.Fatalf("calls %d or tree changed", len(f.Calls))
	}
}
