package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Atom is one extracted claim with its evidence.
type Atom struct {
	Claim      string   `json:"claim"`
	Evidence   string   `json:"evidence"`
	Stance     string   `json:"stance"`
	Locator    string   `json:"locator"`
	Tags       []string `json:"tags"`
	Confidence *float64 `json:"confidence,omitempty"`
	Rendering  string   `json:"rendering,omitempty"`
}

// Drop reports an atom removed by CheckAtoms. Index is its position in the model's list.
type Drop struct {
	Index        int
	Code, Reason string
}

func atomCap(it Item, caps vault.CapsConfig) int {
	if vault.IsAesthetic(it.Medium) {
		return caps.AtomsPerAestheticInput
	}
	return caps.AtomsPerAssertsInput
}

func checkAtom(a Atom, it Item, lines []string) (string, string) {
	switch a.Stance {
	case "asserts", "crafts", "reads":
	default:
		return "BAD_STANCE", "stance must be asserts, crafts or reads"
	}
	if a.Claim == "" || strings.Contains(a.Claim, "\n") {
		return "BAD_CLAIM", "claim must be one non-empty line"
	}
	if a.Evidence == "" || !strings.Contains(it.Plain, a.Evidence) {
		return "EVIDENCE_NOT_FOUND", "evidence is not an exact quote of the input"
	}
	if a.Locator == "" {
		if vault.IsAesthetic(it.Medium) || it.Medium == "thread" || it.Medium == "code" {
			return "LOCATOR_REQUIRED", "locator is required for this medium"
		}
	} else {
		loc, err := vault.ParseLocator(a.Locator)
		if err != nil {
			return "BAD_LOCATOR", err.Error()
		}
		if it.Medium == "thread" && loc.Kind != "comment" {
			return "BAD_LOCATOR", "thread needs a comment locator"
		}
		if it.Medium == "code" && loc.Kind != "lines" {
			return "BAD_LOCATOR", "code needs a lines locator"
		}
		if from, to, ok := loc.LineRange(); ok {
			if from < 1 || to < from || to > len(lines) {
				return "BAD_LOCATOR", "line range outside the input"
			}
			if !strings.Contains(strings.Join(lines[from-1:to], "\n"), a.Evidence) {
				return "BAD_LOCATOR", "evidence is not inside the located lines"
			}
		}
	}
	if a.Confidence != nil {
		if a.Stance != "asserts" {
			return "CONFIDENCE_NOT_ASSERTS", "confidence is only allowed on asserts atoms"
		}
		if c := *a.Confidence; c < 0 || c > 1 {
			return "BAD_CONFIDENCE", "confidence must be within 0..1"
		}
	}
	return "", ""
}

// CheckAtoms enforces the atom rules and caps in code.
func CheckAtoms(raw []Atom, it Item, caps vault.CapsConfig) ([]Atom, []Drop) {
	lines := vault.InboxLines(it.Stored, it.Medium)
	limit := atomCap(it, caps)
	var kept []Atom
	var drops []Drop
	for i, a := range raw {
		if code, reason := checkAtom(a, it, lines); code != "" {
			drops = append(drops, Drop{Index: i, Code: code, Reason: reason})
			continue
		}
		if it.Medium == "own" && a.Stance == "asserts" && a.Confidence == nil {
			c := 0.3
			a.Confidence = &c
		}
		var tags []string
		seen := map[string]bool{}
		for _, t := range a.Tags {
			s := vault.Slug(t, 32)
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			tags = append(tags, s)
		}
		a.Tags = tags
		if len(kept) >= limit {
			drops = append(drops, Drop{Index: i, Code: "CAP", Reason: fmt.Sprintf("more than %d atoms for this input", limit)})
			continue
		}
		kept = append(kept, a)
	}
	return kept, drops
}

// Extract asks the model for atoms and checks them in code.
func Extract(ctx context.Context, p provider.Provider, pr provider.Prompt, it Item, caps vault.CapsConfig) ([]Atom, []Drop, provider.Usage, error) {
	var u provider.Usage
	if utf8.RuneCountInString(it.Plain) > caps.CharsPerCall {
		return nil, nil, u, errs.New(errs.TooLong, "input exceeds chars_per_call cap", "shorten the input or raise caps.chars_per_call")
	}
	class := Class(it.Medium, it.Note != "", it.Hunch != "")
	req := provider.Request{
		Role:      "extract",
		Medium:    it.Medium,
		System:    pr.Text,
		User:      Header(it) + fmt.Sprintf("\nClass: %s. Return 1 to %d atoms. Copy evidence exactly, without the line-number prefixes.\n\n", class, atomCap(it, caps)) + FencedText(it),
		MaxTokens: 2000,
	}
	var out struct{ Atoms []Atom }
	u, err := provider.CallJSON(ctx, p, req, &out, func() error {
		if len(out.Atoms) == 0 {
			return errors.New("atoms must contain at least one atom")
		}
		return nil
	})
	if err != nil {
		if errs.CodeOf(err) != errs.ProviderError {
			err = errs.New(errs.ProviderError, err.Error(), "")
		}
		return nil, nil, u, err
	}
	kept, drops := CheckAtoms(out.Atoms, it, caps)
	return kept, drops, u, nil
}
