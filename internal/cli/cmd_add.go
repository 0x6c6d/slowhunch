package cli

import (
	"context"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	_ "github.com/0x6c6d/slowhunch/internal/provider/exec"
	_ "github.com/0x6c6d/slowhunch/internal/provider/openai"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("add", "add <text|file|-> [--as M] [--url U] [--title T] [--author A] [--note N] [--hunch ID] [-force]: ingest one item", cmdAdd)
}

func cmdAdd(env *Env, args []string) (any, error) {
	env.JSON = true // add always reports JSON
	fs := newFlagSet("add")
	as := fs.String("as", "", "")
	url := fs.String("url", "", "")
	title := fs.String("title", "", "")
	author := fs.String("author", "", "")
	note := fs.String("note", "", "")
	hunch := fs.String("hunch", "", "")
	force := fs.Bool("force", false, "")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "add needs exactly one text, file or -", "usage: slowhunch add <text|file|-> [--as M] [--title T] [-force]")
	}
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	type pkey struct{ role, medium string }
	cache := map[pkey]provider.Provider{}
	providerFor := func(role, medium string) (provider.Provider, error) {
		k := pkey{role, medium}
		if p, ok := cache[k]; ok {
			return p, nil
		}
		p, err := provider.ForRole(cfg, role, medium)
		if err != nil {
			return nil, err
		}
		cache[k] = p
		return p, nil
	}
	for _, role := range []string{"gate", "extract", "place"} {
		if _, err := providerFor(role, ""); err != nil {
			return nil, err
		}
	}
	prompts, err := provider.LoadPrompts(v.Root)
	if err != nil {
		return nil, err
	}
	var committer vault.Committer
	if !env.NoCommit && cfg.Git.Commit && gitops.Available() {
		committer = gitops.New(v.Root)
	}
	authorFor := func(role, medium string) string {
		eff := provider.Effective(cfg, role, medium)
		model := eff.Model
		if model == "" {
			model = "unknown"
		}
		return "agent:" + eff.Name + "/" + model
	}
	d := pipeline.Deps{
		Vault: v, Config: cfg, ProviderFor: providerFor, AuthorFor: authorFor, Prompts: prompts, Committer: committer,
		Now: env.Now, Author: authorFor("extract", ""),
	}
	o := pipeline.Options{
		Arg: pos[0], Stdin: env.Stdin, Medium: *as, URL: *url, Title: *title, Author: *author,
		Note: *note, Hunch: *hunch, Force: *force, DryRun: env.DryRun, NoCommit: env.NoCommit,
		PDFCommand: cfg.PDF.Command,
	}
	rep, err := pipeline.Add(context.Background(), d, o)
	if err != nil {
		return nil, err
	}
	return rep, nil
}
