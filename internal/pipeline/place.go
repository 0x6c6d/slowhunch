package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/search"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Neighbour is an existing note shown to the place step.
type Neighbour struct {
	ID        string
	Cosine    float64
	Excerpt   string
	Aesthetic bool
	Stance    string
	Tags      []string
	Far       bool
}

// Verdict is the model's placement advice.
type Verdict struct {
	Verdict string `json:"verdict"`
	Target  string `json:"target"`
	Reason  string `json:"reason"`
}

// Decision is the code's placement decision.
// Action is one of new, extends, duplicate, contradicts, rereads.
type Decision struct {
	Action, Target string
	Cosine         float64
	Echo           bool
	EchoOf         string
	Reason         string
}

// Candidate is an existing note (from notes/ only).
type Candidate struct {
	ID, Text, Evidence, Stance string
	Tags                       []string
	Aesthetic                  bool
}

// CapExcerpt returns at most n runes of s.
func CapExcerpt(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func atomText(a Atom) string { return a.Claim + "\n" + a.Evidence }

// BuildNeighbours finds the near neighbours of a among cands plus the most
// similar note of the other class, flagged Far.
func BuildNeighbours(a Atom, aesthetic bool, cands []Candidate, cfg vault.PlaceConfig) []Neighbour {
	if len(cands) == 0 {
		return nil
	}
	docs := make([]search.Doc, 0, len(cands))
	byID := make(map[string]Candidate, len(cands))
	for _, c := range cands {
		if _, dup := byID[c.ID]; dup {
			continue
		}
		byID[c.ID] = c
		docs = append(docs, search.Doc{ID: c.ID, Text: c.Text})
	}
	ix := search.Build(docs)
	near, far := search.Neighbours(ix, atomText(a), cfg.Near, func(id string) bool { return byID[id].Aesthetic }, aesthetic)
	mk := func(h search.Hit, isFar bool) Neighbour {
		c := byID[h.ID]
		return Neighbour{
			ID:        c.ID,
			Cosine:    h.Score,
			Excerpt:   CapExcerpt(c.Evidence, cfg.ExcerptChars),
			Aesthetic: c.Aesthetic,
			Stance:    c.Stance,
			Tags:      append([]string(nil), c.Tags...),
			Far:       isFar,
		}
	}
	out := make([]Neighbour, 0, len(near)+1)
	for _, h := range near {
		out = append(out, mk(h, false))
	}
	if far != nil {
		out = append(out, mk(*far, true))
	}
	return out
}

func shareTag(a, b []string) bool {
	set := map[string]bool{}
	for _, t := range a {
		set[t] = true
	}
	for _, t := range b {
		if set[t] {
			return true
		}
	}
	return false
}

// Decide turns the model's advice into a decision; the code decides.
func Decide(a Atom, aesthetic bool, v Verdict, nbs []Neighbour, cfg vault.PlaceConfig) Decision {
	newDecision := func() Decision {
		d := Decision{Action: "new", Reason: v.Reason}
		if !aesthetic {
			return d
		}
		best := -1
		for i, n := range nbs {
			if n.Far {
				continue
			}
			if best < 0 || n.Cosine > nbs[best].Cosine {
				best = i
			}
		}
		if best >= 0 {
			if c := nbs[best].Cosine; c >= cfg.EchoLow && c < cfg.EchoHigh {
				d.Echo = true
				d.EchoOf = nbs[best].ID
				d.Cosine = c
			}
		}
		return d
	}
	switch v.Verdict {
	case "duplicate", "extends", "contradicts":
	default:
		return newDecision()
	}
	var target *Neighbour
	for i := range nbs {
		if nbs[i].ID == v.Target && v.Target != "" {
			target = &nbs[i]
			break
		}
	}
	if target == nil {
		return newDecision()
	}
	cos := target.Cosine
	d := Decision{Target: target.ID, Cosine: cos, Reason: v.Reason}
	switch v.Verdict {
	case "duplicate":
		switch {
		case cos < cfg.DuplicateCosine:
			d.Action = "extends"
		case a.Stance == "reads":
			d.Action = "rereads"
		default:
			d.Action = "duplicate"
		}
	case "contradicts":
		if shareTag(a.Tags, target.Tags) || cos >= cfg.ContradictCosine {
			d.Action = "contradicts"
		} else {
			d.Action = "extends"
		}
	default:
		d.Action = "extends"
	}
	return d
}

func placeUser(a Atom, it Item, nbs []Neighbour) string {
	var b strings.Builder
	b.WriteString("New atom (stance: " + a.Stance + "):\nClaim:\n")
	b.WriteString(intake.Fence(a.Claim, it.Delim))
	b.WriteString("\nEvidence:\n")
	b.WriteString(intake.Fence(a.Evidence, it.Delim))
	b.WriteString("\n\nNeighbours (id, cosine, far, excerpt):\n")
	for _, n := range nbs {
		fmt.Fprintf(&b, "\nid: %s\ncosine: %.2f\nfar: %t\nexcerpt:\n%s\n", n.ID, n.Cosine, n.Far, intake.Fence(n.Excerpt, it.Delim))
	}
	b.WriteString("\nReturn {\"verdict\": \"new|extends|duplicate|contradicts\", \"target\": \"<neighbour id or empty>\", \"reason\": \"...\"}.\n")
	return b.String()
}

// Place asks the model for one verdict about one atom and decides in code.
func Place(ctx context.Context, p provider.Provider, pr provider.Prompt, a Atom, it Item, nbs []Neighbour, cfg vault.PlaceConfig) (Decision, provider.Usage, error) {
	var u provider.Usage
	if len(nbs) == 0 {
		return Decision{Action: "new"}, u, nil
	}
	req := provider.Request{
		Role:      "place",
		Medium:    it.Medium,
		System:    pr.Text,
		User:      placeUser(a, it, nbs),
		MaxTokens: 500,
	}
	var v Verdict
	u, err := provider.CallJSON(ctx, p, req, &v, func() error {
		switch v.Verdict {
		case "new", "extends", "duplicate", "contradicts":
			return nil
		}
		return errors.New("verdict must be new, extends, duplicate or contradicts")
	})
	if err != nil {
		if errs.CodeOf(err) != errs.ProviderError {
			err = errs.New(errs.ProviderError, err.Error(), "")
		}
		return Decision{}, u, err
	}
	return Decide(a, vault.IsAesthetic(it.Medium), v, nbs, cfg), u, nil
}
