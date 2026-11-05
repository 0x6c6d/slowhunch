package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/report"
)

// rejectFake rejects every input at the gate; extract and place answer as addFake.
func rejectFake() *fake.Fake {
	f := addFake()
	inner := f.Handler
	f.Handler = func(req provider.Request) (provider.Response, error) {
		if req.Role == "gate" {
			return provider.Response{Text: `{"keep": false, "reason": "boring"}`}, nil
		}
		return inner(req)
	}
	return f
}

// wildcardBatch writes n distinct article entries and returns the batch file
// path and the entry texts.
func wildcardBatch(t *testing.T, n int) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	var lines, texts []string
	for i := 0; i < n; i++ {
		text := fmt.Sprintf("Entry %d says the river flooded the old mill in spring number %d.\n", i, i*7+3)
		name := fmt.Sprintf("e%d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("%s|https://example.invalid/%d|Mill %d|Ada Lovelace", name, i, i))
		texts = append(texts, text)
	}
	p := filepath.Join(dir, "batch.txt")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, texts
}

func wildcardKey(text string) string {
	sum := sha256.Sum256([]byte(intake.TextHash(intake.Normalize(text, "article"))))
	return hex.EncodeToString(sum[:])
}

func runWildcardBatch(t *testing.T, n int, share float64) (*BatchResult, []string, []*report.Report) {
	t.Helper()
	v, r := addVault(t)
	d := addDeps(v, r, rejectFake())
	d.Config.Place.WildcardShare = share
	from, texts := wildcardBatch(t, n)
	res, err := AddBatch(context.Background(), d, from, Options{Medium: "article"})
	if res == nil {
		t.Fatalf("no result: %v", err)
	}
	return res, texts, res.Items
}

func TestWildcardTen(t *testing.T) {
	v, r := addVault(t)
	d := addDeps(v, r, rejectFake())
	from, texts := wildcardBatch(t, 10)
	res, err := AddBatch(context.Background(), d, from, Options{Medium: "article"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts["written"] != 1 || res.Counts["wildcards"] != 10 || res.Counts["wildcards_kept"] != 1 {
		t.Fatalf("counts %v", res.Counts)
	}
	want := 0
	for i, tx := range texts {
		if wildcardKey(tx) < wildcardKey(texts[want]) {
			want = i
		}
	}
	nKept := 0
	for i, rep := range res.Items {
		if len(rep.Wildcards) != 1 {
			t.Fatalf("entry %d wildcards %+v", i, rep.Wildcards)
		}
		w := rep.Wildcards[0]
		if w.Item != fmt.Sprintf("line %d: e%d.txt", i+1, i) || w.Reason != "boring" {
			t.Fatalf("entry %d wildcard %+v", i, w)
		}
		if w.Kept {
			nKept++
			if i != want {
				t.Fatalf("kept entry %d, want %d (lowest sha256(text_hash))", i, want)
			}
			if len(rep.Atoms) == 0 {
				t.Fatalf("kept wildcard wrote no atoms: %+v", rep)
			}
			for _, a := range rep.Atoms {
				doc := loadDoc(t, v, "notes/"+unlink(a.ID)+".md")
				if !contains(doc.GetList("tags"), "wildcard") {
					t.Fatalf("atom %s tags %v", a.ID, doc.GetList("tags"))
				}
			}
			src := loadDoc(t, v, "sources/"+unlink(rep.Source)+".md")
			if src.GetString("text_hash") != intake.TextHash(intake.Normalize(texts[want], "article")) {
				t.Fatal("written source is not the chosen entry")
			}
		} else if len(rep.Atoms) != 0 {
			t.Fatalf("rejected entry %d wrote atoms", i)
		}
	}
	if nKept != 1 {
		t.Fatalf("kept %d", nKept)
	}
	if n := len(noteFiles(t, v)); n != 1 {
		t.Fatalf("notes %d", n)
	}
	srcs, _ := os.ReadDir(filepath.Join(v.Root, "sources"))
	if len(srcs) != 1 {
		t.Fatalf("sources %d", len(srcs))
	}
}

func TestWildcardSingle(t *testing.T) {
	v, r := addVault(t)
	before, head := treeHash(t, v.Root), headOf(t, v.Root)
	rep := mustAdd(t, v, r, rejectFake(), Options{Arg: addArticle, Medium: "article", Title: "Flood"})
	if treeHash(t, v.Root) != before || headOf(t, v.Root) != head {
		t.Fatal("a single rejected item was written")
	}
	if len(rep.Wildcards) != 1 || rep.Wildcards[0].Kept || rep.Wildcards[0].Reason != "boring" || rep.Wildcards[0].Item == "" {
		t.Fatalf("wildcards %+v", rep.Wildcards)
	}
	if rep.Counts["wildcards"] != 1 || rep.Counts["wildcards_kept"] != 0 {
		t.Fatalf("counts %v", rep.Counts)
	}
	if len(rep.Atoms) != 0 || rep.Counts["rejected"] != 1 {
		t.Fatalf("report %+v", rep)
	}
}

func TestWildcardShare(t *testing.T) {
	for _, c := range []struct {
		share float64
		kept  int
	}{{0.5, 2}, {0, 0}} {
		res, texts, items := runWildcardBatch(t, 4, c.share)
		if res.Counts["wildcards"] != 4 || res.Counts["wildcards_kept"] != c.kept || res.Counts["written"] != c.kept {
			t.Fatalf("share %v counts %v", c.share, res.Counts)
		}
		// The kept ones are the lowest keys.
		var keys []string
		for _, tx := range texts {
			keys = append(keys, wildcardKey(tx))
		}
		for i, rep := range items {
			rank := 0
			for _, k := range keys {
				if k < keys[i] {
					rank++
				}
			}
			if rep.Wildcards[0].Kept != (rank < c.kept) {
				t.Fatalf("share %v entry %d kept=%v rank %d", c.share, i, rep.Wildcards[0].Kept, rank)
			}
		}
	}
}
