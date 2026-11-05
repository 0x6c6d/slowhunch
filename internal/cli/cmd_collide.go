package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/synth"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("collide", "pair distant notes and hunches and write proposals [--limit N]", cmdCollide)
}

const collideUsage = "usage: slowhunch collide [--limit N] [--dry-run]"

type collidePair struct {
	Items    []string `json:"items"`
	Cosine   float64  `json:"cosine"`
	Score    float64  `json:"score"`
	Reserved bool     `json:"reserved"`
}

type collideDryResult struct {
	DryRun bool           `json:"dry_run"`
	Pairs  []collidePair  `json:"pairs"`
	Counts map[string]int `json:"counts"`
	Capped bool           `json:"capped"`
}

type collideResult struct {
	*report.Report
	Capped bool
}

func (r collideResult) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(r.Report)
	if err != nil {
		return nil, err
	}
	c, _ := json.Marshal(r.Capped)
	return append(append(b[:len(b)-1], []byte(`,"capped":`)...), append(c, '}')...), nil
}

// proposalWrite is one proposal planned by a collide run.
type proposalWrite struct {
	id  string
	doc *frontmatter.Doc
}

func cmdCollide(env *Env, args []string) (any, error) {
	fs := newFlagSet("collide")
	limit := fs.Int("limit", 0, "")
	dry := fs.Bool("dry-run", false, "")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "collide takes no arguments", collideUsage)
	}
	limitSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "limit" {
			limitSet = true
		}
	})
	if limitSet && *limit < 1 {
		return nil, errs.New(errs.BadInput, "--limit must be >= 1", collideUsage)
	}
	dryRun := env.DryRun || *dry
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	if !dryRun {
		if _, err := env.prepareRun(v, cfg); err != nil {
			return nil, err
		}
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	sel := synth.Select(synth.Pool(ix), synth.ExistingPairs(ix), cfg.Collide, *limit)
	counts := map[string]int{"pairs_candidates": sel.Candidates, "pairs_tried": 0, "pairs_kept": 0}

	if dryRun {
		out := collideDryResult{DryRun: true, Pairs: []collidePair{}, Counts: counts, Capped: sel.Capped}
		for _, p := range sel.Pairs {
			out.Pairs = append(out.Pairs, collidePair{Items: []string{report.Link(p.A), report.Link(p.B)},
				Cosine: p.Cosine, Score: p.Score, Reserved: p.Reserved})
		}
		return out, nil
	}

	prompts, err := provider.LoadPrompts(v.Root)
	if err != nil {
		return nil, err
	}
	pr, ok := prompts["collide"]
	if !ok {
		return nil, errs.New(errs.NotFound, "no collide prompt", "run slowhunch init to restore the prompts")
	}
	cache := map[string]provider.Provider{}
	providerFor := func(medium string) (provider.Provider, error) {
		if p, ok := cache[medium]; ok {
			return p, nil
		}
		p, err := provider.ForRole(cfg, "collide", medium)
		if err != nil {
			return nil, err
		}
		cache[medium] = p
		return p, nil
	}
	authorFor := func(medium string) string {
		eff := provider.Effective(cfg, "collide", medium)
		model := eff.Model
		if model == "" {
			model = "unknown"
		}
		return "agent:" + eff.Name + "/" + model
	}

	rep := report.New(vault.NewRunID(env.Now()))
	rep.Counts = counts
	if sel.Capped {
		rep.Errors = append(rep.Errors, report.ItemError{Item: "collide", Code: errs.CapReached,
			Reason: fmt.Sprintf("max_pairs %d of %d candidates", len(sel.Pairs), sel.Candidates)})
	}

	today := env.today()
	taken := map[string]bool{}
	var writes []proposalWrite
	ctx := context.Background()
	for _, p := range sel.Pairs {
		item := p.A + "+" + p.B
		pairErr := func(err error) {
			code, msg := errs.CodeOf(err), err.Error()
			if e, ok := err.(*errs.Error); ok {
				msg = e.Message
			}
			if code == "" {
				code = errs.ProviderError
			}
			rep.Errors = append(rep.Errors, report.ItemError{Item: item, Code: code, Reason: msg})
		}
		medium := ix.MediumOf(p.A)
		pv, err := providerFor(medium)
		if err != nil {
			return nil, err
		}
		req, err := synth.PairRequest(ix, p, pr, medium, cfg.Place.ExcerptChars)
		if err != nil {
			pairErr(err)
			continue
		}
		counts["pairs_tried"]++
		r, u, err := synth.Propose(ctx, pv, req)
		rep.AddCost("collide", report.Usage{TokensIn: u.TokensIn, TokensOut: u.TokensOut})
		if err != nil {
			pairErr(err)
			continue
		}
		if !r.Kept() {
			continue
		}
		id := vault.UniqueID(synth.ProposalSlug(r.Idea, p.A, p.B), func(c string) bool {
			if taken[c] {
				return true
			}
			if _, ok := ix.Get(c); ok {
				return true
			}
			return v.Exists("proposals/" + c + ".md")
		})
		var bears []string
		for _, x := range []string{p.A, p.B} {
			if e, ok := ix.Get(x); ok && e.Type == "hunch" {
				bears = append(bears, x)
			}
		}
		d := synth.ProposalDoc(id, p, r, bears, authorFor(medium), today)
		if err := vault.Validate(d, id); err != nil {
			pairErr(err)
			continue
		}
		taken[id] = true
		counts["pairs_kept"]++
		writes = append(writes, proposalWrite{id: id, doc: d})
		rep.Collisions = append(rep.Collisions, report.Collision{
			Items: []string{report.Link(p.A), report.Link(p.B)}, Proposal: report.Link(id)})
	}
	counts["errors"] = len(rep.Errors)
	rep.Errors = append(rep.Errors, env.prep.itemErrors()...)

	if len(writes) == 0 {
		rep.Git = report.GitInfo{Skipped: "nothing-kept", Pulled: env.prep.pulled()}
		return collideResult{rep, sel.Capped}, nil
	}
	o := vault.TxnOptions{RunID: rep.RunID, Now: env.Now}
	noCommit := env.NoCommit || !cfg.Git.Commit || !gitops.Available()
	if noCommit {
		o.NoCommit = true
	} else {
		o.Committer = gitops.New(v.Root)
	}
	o.PromptVersions = provider.Versions(prompts)
	txn, err := v.Begin("collide", o)
	if err != nil {
		return nil, err
	}
	var paths, ids []string
	for _, w := range writes {
		paths = append(paths, "proposals/"+w.id+".md")
		ids = append(ids, w.id)
	}
	if err := txn.Touch(paths...); err != nil {
		return nil, err
	}
	for i, w := range writes {
		if _, err := txn.PutDoc(paths[i], w.doc, ""); err != nil {
			return nil, err
		}
	}
	hash, err := txn.Commit(ids, fmt.Sprintf("proposals: %d", len(writes)))
	if err != nil {
		return nil, err
	}
	switch {
	case env.NoCommit || !cfg.Git.Commit:
		rep.Git = report.GitInfo{Skipped: "no-commit"}
	case !gitops.Available():
		rep.Git = report.GitInfo{Skipped: "no-git"}
	default:
		rep.Git = report.GitInfo{Commit: hash}
	}
	rep.Git.Pulled = env.prep.pulled()
	if err := rep.Save(v); err != nil {
		return nil, err
	}
	return collideResult{rep, sel.Capped}, nil
}
