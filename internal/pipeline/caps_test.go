package pipeline

import (
	"context"
	"os"
	"path/filepath"
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
