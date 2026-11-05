package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
)

const delim = "SLOWHUNCH-INTAKE-0123456789abcdef"

var pr = provider.Prompt{Role: "gate", Version: "gate-1", Text: "sys"}

func item(medium string) Item {
	return Item{Medium: medium, Title: "T", Plain: "Rates rose.", Stored: []byte("Rates rose.\n"), Delim: delim}
}

func keepFake(keep string) *fake.Fake {
	return &fake.Fake{Replies: map[string][]string{"gate": {`{"keep": ` + keep + `, "reason": "r"}`}}}
}

func checkUser(t *testing.T, medium, want, other string) {
	f := keepFake("true")
	keep, _, _, err := Gate(context.Background(), f, pr, item(medium))
	if err != nil || !keep {
		t.Fatalf("keep=%v err=%v", keep, err)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("calls=%d", len(f.Calls))
	}
	u := f.Calls[0].User
	if !strings.Contains(u, want) || strings.Contains(u, other) || !strings.Contains(u, delim) || !strings.Contains(u, "Rates rose.") {
		t.Fatalf("bad user message: %q", u)
	}
}

func TestGateFactual(t *testing.T)   { checkUser(t, "article", factualQuestion, aestheticQuestion) }
func TestGateAesthetic(t *testing.T) { checkUser(t, "poem", aestheticQuestion, factualQuestion) }

func TestGateOwnerSkips(t *testing.T) {
	cases := []Item{item("own"), item("article"), item("article")}
	cases[1].Note = "n"
	cases[2].Hunch = "h"
	for _, it := range cases {
		f := keepFake("false")
		keep, reason, _, err := Gate(context.Background(), f, pr, it)
		if err != nil || !keep || reason != "owner-steered" || len(f.Calls) != 0 {
			t.Fatalf("%+v: keep=%v reason=%q err=%v calls=%d", it, keep, reason, err, len(f.Calls))
		}
	}
}

func TestGateForce(t *testing.T) {
	f := keepFake("false")
	it := item("article")
	it.Force = true
	keep, reason, _, err := Gate(context.Background(), f, pr, it)
	if err != nil || !keep || reason != "forced" || len(f.Calls) != 0 {
		t.Fatalf("keep=%v reason=%q err=%v", keep, reason, err)
	}
}

func TestGateProviderError(t *testing.T) {
	f := &fake.Fake{Handler: func(provider.Request) (provider.Response, error) { return provider.Response{}, errors.New("boom") }}
	keep, _, _, err := Gate(context.Background(), f, pr, item("article"))
	if keep || errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("keep=%v err=%v", keep, err)
	}
}

func TestGateRejects(t *testing.T) {
	keep, _, _, err := Gate(context.Background(), keepFake("false"), pr, item("article"))
	if keep || err != nil {
		t.Fatalf("keep=%v err=%v", keep, err)
	}
}

func TestGateSecret(t *testing.T) {
	f := keepFake("true")
	it := item("article")
	it.Plain = "token sk-" + strings.Repeat("a", 20)
	it.Stored = []byte(it.Plain + "\n")
	for _, force := range []bool{false, true} {
		it.Force = force
		keep, _, _, err := Gate(context.Background(), f, pr, it)
		if keep || errs.CodeOf(err) != errs.BadInput || len(f.Calls) != 0 {
			t.Fatalf("force=%v keep=%v err=%v", force, keep, err)
		}
	}
}
