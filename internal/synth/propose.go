package synth

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Reply is the collide model's answer for one pair.
type Reply struct {
	Idea         string `json:"idea"`
	CheapestTest string `json:"cheapest_test"`
	Keep         *bool  `json:"keep"`
}

// Kept reports whether the reply keeps the pair.
func (r Reply) Kept() bool { return r.Keep != nil && *r.Keep }

// Validate checks the reply in code. The error never repeats reply text.
func (r *Reply) Validate() error {
	if r.Keep == nil {
		return errors.New("keep must be a boolean")
	}
	idea := strings.Join(strings.Fields(r.Idea), " ")
	test := strings.Join(strings.Fields(r.CheapestTest), " ")
	if n := utf8.RuneCountInString(idea); n < 1 || n > 200 {
		return errors.New("idea must be 1-200 characters")
	}
	if n := utf8.RuneCountInString(test); n < 1 || n > 400 {
		return errors.New("cheapest_test must be 1-400 characters")
	}
	if intake.ContainsSecret(r.Idea) || intake.ContainsSecret(r.CheapestTest) {
		return errors.New("reply contains a secret-like string")
	}
	r.Idea, r.CheapestTest = idea, test
	return nil
}

func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// itemText is the excerpt_only view of one item: its fields and a body excerpt
// cut to excerptChars. A whole file is never sent.
func itemText(ix *vault.Index, id string, excerptChars int) string {
	e, ok := ix.Get(id)
	if !ok {
		return "id: " + id
	}
	var b strings.Builder
	line := func(k, v string) {
		v = strings.Join(strings.Fields(v), " ")
		b.WriteString(k + ": " + v + "\n")
	}
	line("id", id)
	line("type", e.Type)
	line("medium", ix.MediumOf(id))
	if e.Type == "note" {
		line("stance", e.Doc.GetString("stance"))
		line("claim", cut(e.Doc.GetString("claim"), excerptChars))
	} else {
		line("stance", "")
		line("question", cut(e.Doc.GetString("question"), excerptChars))
	}
	line("excerpt", cut(strings.TrimSpace(e.Doc.Body), excerptChars))
	return strings.TrimRight(b.String(), "\n")
}

// PairRequest builds the collide request for p: both items fenced together.
func PairRequest(ix *vault.Index, p Pair, pr provider.Prompt, medium string, excerptChars int) (provider.Request, error) {
	text := "item A\n" + itemText(ix, p.A, excerptChars) + "\n\nitem B\n" + itemText(ix, p.B, excerptChars)
	if intake.ContainsSecret(text) {
		return provider.Request{}, errs.New(errs.BadInput, "pair contains a secret-like string; nothing was sent", "remove the secret from the item")
	}
	delim, err := intake.NewDelimiter(text)
	if err != nil {
		return provider.Request{}, err
	}
	return provider.Request{
		Role:      "collide",
		Medium:    medium,
		System:    pr.Text,
		User:      "Two vault items follow. Decide whether together they suggest a new, testable idea.\n\n" + intake.Fence(text, delim),
		MaxTokens: 500,
	}, nil
}

// Propose asks the model about one pair and validates the reply.
func Propose(ctx context.Context, pv provider.Provider, req provider.Request) (Reply, provider.Usage, error) {
	var r Reply
	u, err := provider.CallJSON(ctx, pv, req, &r, r.Validate)
	if err != nil {
		if errs.CodeOf(err) == "" {
			err = errs.New(errs.ProviderError, err.Error(), "")
		}
		return Reply{}, u, err
	}
	return r, u, nil
}

// ProposalSlug turns an idea into a proposal id base: only [a-z0-9-], at most
// 60 characters, never empty (fallback collide-<a>-<b>, then "collide").
func ProposalSlug(idea, a, b string) string {
	if s := vault.Slug(idea, 60); s != "" {
		return s
	}
	if s := vault.Slug("collide-"+a+"-"+b, 60); s != "" {
		return s
	}
	return "collide"
}

// ProposalDoc assembles a new open proposal for a kept pair. The item ids
// come from the pair, never from the reply.
func ProposalDoc(id string, p Pair, r Reply, bearsOn []string, author, today string) *frontmatter.Doc {
	if bearsOn == nil {
		bearsOn = []string{}
	}
	f := func(k string, v any) frontmatter.Field { return frontmatter.Field{Key: k, Value: v} }
	d := &frontmatter.Doc{}
	d.Fields = []frontmatter.Field{
		f("id", id), f("type", "proposal"),
		f("created", today), f("updated", today),
		f("author", author), f("rev", "000000"),
		f("tags", []string{}), f("bears_on", bearsOn),
		f("items", []string{p.A, p.B}),
		f("idea", r.Idea), f("cheapest_test", r.CheapestTest),
		f("status", "open"),
	}
	d.Body = "# " + r.Idea + "\n\nItems: [[" + p.A + "]] and [[" + p.B + "]].\n\nCheapest test: " + r.CheapestTest + "\n"
	return d
}
