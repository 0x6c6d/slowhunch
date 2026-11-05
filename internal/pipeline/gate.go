package pipeline

import (
	"context"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/provider"
)

// GateVerdict is the gate model's answer.
type GateVerdict struct {
	Keep   bool   `json:"keep"`
	Reason string `json:"reason"`
}

const (
	factualQuestion   = "Does this text make a specific, checkable claim with support that is worth keeping as evidence? Answer keep true or false."
	aestheticQuestion = "Is there a line or passage here worth keeping for its craft or for how it reads? Answer keep true or false."
)

// Gate decides whether an item is worth keeping. It fails closed.
func Gate(ctx context.Context, p provider.Provider, pr provider.Prompt, it Item) (keep bool, reason string, u provider.Usage, err error) {
	if intake.ContainsSecret(it.Plain) {
		return false, "", u, errs.New(errs.BadInput, "input contains a secret-like string; nothing was sent", "remove the secret and add again")
	}
	if it.Force {
		return true, "forced", u, nil
	}
	class := Class(it.Medium, it.Note != "", it.Hunch != "")
	if class == "owner" {
		return true, "owner-steered", u, nil
	}
	q := factualQuestion
	if class == "aesthetic" {
		q = aestheticQuestion
	}
	req := provider.Request{
		Role:      "gate",
		Medium:    it.Medium,
		System:    pr.Text,
		User:      Header(it) + "\n" + q + "\n\n" + FencedText(it),
		MaxTokens: 300,
	}
	var v GateVerdict
	u, err = provider.CallJSON(ctx, p, req, &v, nil)
	if err != nil {
		if errs.CodeOf(err) != errs.ProviderError {
			err = errs.New(errs.ProviderError, err.Error(), "")
		}
		return false, "", u, err
	}
	return v.Keep, v.Reason, u, nil
}
