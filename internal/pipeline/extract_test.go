package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

var exPrompt = provider.Prompt{Role: "extract", Version: "extract-1", Text: "sys"}

func exItem(medium, text string) Item {
	st := vault.FormatInbox(text, medium)
	return Item{Medium: medium, Title: "T", Stored: st, Plain: vault.InboxPlain(st, medium), Delim: delim}
}

func caps() vault.CapsConfig {
	return vault.CapsConfig{AtomsPerAssertsInput: 5, AtomsPerAestheticInput: 3, CharsPerCall: 1000}
}

const poem = "The trees are coming into leaf\nLike something almost being said;"

func codes(d []Drop) []string {
	var s []string
	for _, x := range d {
		s = append(s, x.Code)
	}
	return s
}

func wantCodes(t *testing.T, d []Drop, want ...string) {
	t.Helper()
	got := codes(d)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("drop codes %v, want %v", got, want)
	}
}

func TestExtractEvidenceDrop(t *testing.T) {
	it := exItem("poem", poem)
	kept, d := CheckAtoms([]Atom{{Claim: "c", Evidence: "Trees are leafing", Stance: "crafts", Locator: "line 1"}}, it, caps())
	if len(kept) != 0 {
		t.Fatal("kept")
	}
	wantCodes(t, d, "EVIDENCE_NOT_FOUND")
}

func TestExtractBadLocator(t *testing.T) {
	it := exItem("poem", poem)
	_, d := CheckAtoms([]Atom{
		{Claim: "c", Evidence: "The trees", Stance: "crafts", Locator: "lines 5-6"},
		{Claim: "c", Evidence: "The trees", Stance: "crafts", Locator: "line 2"},
		{Claim: "c", Evidence: "The trees", Stance: "crafts"},
	}, it, caps())
	wantCodes(t, d, "BAD_LOCATOR", "BAD_LOCATOR", "LOCATOR_REQUIRED")
	for _, x := range d {
		if x.Reason == "" {
			t.Fatal("empty reason")
		}
	}
}

func TestExtractConfidenceCrafts(t *testing.T) {
	c := 0.5
	bad := 2.0
	it := exItem("article", "Rates rose.")
	_, d := CheckAtoms([]Atom{
		{Claim: "c", Evidence: "Rates", Stance: "crafts", Confidence: &c},
		{Claim: "c", Evidence: "Rates", Stance: "asserts", Confidence: &bad},
	}, it, caps())
	wantCodes(t, d, "CONFIDENCE_NOT_ASSERTS", "BAD_CONFIDENCE")
}

func TestExtractCaps(t *testing.T) {
	var raw []Atom
	for i := 0; i < 5; i++ {
		raw = append(raw, Atom{Claim: "c", Evidence: "The trees", Stance: "crafts", Locator: "line 1"})
	}
	kept, d := CheckAtoms(raw, exItem("poem", poem), caps())
	if len(kept) != 3 {
		t.Fatalf("kept %d", len(kept))
	}
	wantCodes(t, d, "CAP", "CAP")
	if d[0].Index != 3 || d[1].Index != 4 {
		t.Fatalf("%+v", d)
	}
	raw = nil
	for i := 0; i < 7; i++ {
		raw = append(raw, Atom{Claim: "c", Evidence: "Rates", Stance: "asserts"})
	}
	kept, d = CheckAtoms(raw, exItem("article", "Rates rose."), caps())
	if len(kept) != 5 || len(d) != 2 {
		t.Fatalf("kept %d drops %d", len(kept), len(d))
	}
}

func TestExtractOwnDefaults(t *testing.T) {
	it := exItem("own", "Rates rose.")
	kept, d := CheckAtoms([]Atom{
		{Claim: "c", Evidence: "Rates", Stance: "asserts", Tags: []string{"Interest Rates", "interest rates", "!!", "x"}},
	}, it, caps())
	if len(d) != 0 || len(kept) != 1 {
		t.Fatalf("%+v", d)
	}
	a := kept[0]
	if a.Confidence == nil || *a.Confidence != 0.3 {
		t.Fatalf("confidence %v", a.Confidence)
	}
	if len(a.Tags) != 2 || a.Tags[0] != vault.Slug("Interest Rates", 32) || a.Tags[1] != "x" {
		t.Fatalf("tags %v", a.Tags)
	}
}

func TestExtractCode(t *testing.T) {
	it := exItem("code", "a := 1\nb := 2\nc := 3\nd := 4")
	kept, d := CheckAtoms([]Atom{
		{Claim: "c", Evidence: "b := 2\nc := 3", Stance: "asserts", Locator: "lines 2-3"},
		{Claim: "c", Evidence: "b := 2", Stance: "asserts", Locator: "line 2"},
		{Claim: "c", Evidence: "b := 2", Stance: "asserts"},
	}, it, caps())
	if len(kept) != 1 {
		t.Fatalf("kept %d", len(kept))
	}
	wantCodes(t, d, "BAD_LOCATOR", "LOCATOR_REQUIRED")
}

func TestExtractCharsCap(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"extract": {`{"atoms":[]}`}}}
	c := caps()
	c.CharsPerCall = 5
	_, _, _, err := Extract(context.Background(), f, exPrompt, exItem("article", "Rates rose."), c)
	if errs.CodeOf(err) != errs.TooLong || len(f.Calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(f.Calls))
	}
}

func TestExtractPrompt(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"extract": {
		`{"atoms":[{"claim":"c","evidence":"Rates rose.","stance":"asserts","locator":"","tags":["a"]},{"claim":"c","evidence":"nope","stance":"asserts","locator":"","tags":[]}]}`}}}
	it := exItem("article", "Rates rose.")
	kept, d, _, err := Extract(context.Background(), f, exPrompt, it, caps())
	if err != nil || len(kept) != 1 {
		t.Fatalf("kept=%d err=%v", len(kept), err)
	}
	wantCodes(t, d, "EVIDENCE_NOT_FOUND")
	if len(f.Calls) != 1 || f.Calls[0].Role != "extract" || !strings.Contains(f.Calls[0].User, FencedText(it)) || !strings.Contains(f.Calls[0].User, "Class: factual") {
		t.Fatalf("bad call: %+v", f.Calls)
	}
}
