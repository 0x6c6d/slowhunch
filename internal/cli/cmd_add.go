package cli

import (
	"context"
	"flag"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	_ "github.com/0x6c6d/slowhunch/internal/provider/exec"
	_ "github.com/0x6c6d/slowhunch/internal/provider/openai"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("add", "add <text|file|-> [--as M] [--url U] [--title T] [--author A] [--note N] [--hunch ID] [-force]: ingest one item; add --from <batch.txt>: ingest up to 50 files in one run", cmdAdd)
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
	from := fs.String("from", "", "")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	batch := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "from" {
			batch = true
		}
	})
	if batch {
		if len(pos) != 0 {
			return nil, errs.New(errs.BadInput, "--from cannot be combined with a text, file or -", "usage: slowhunch add --from <batch.txt> [--as M] [--note N] [--hunch ID] [-force]")
		}
		if *from == "" {
			return nil, errs.New(errs.BadInput, "--from needs a batch file", "usage: slowhunch add --from <batch.txt>")
		}
		var bad []string
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "url" || f.Name == "title" || f.Name == "author" {
				bad = append(bad, "--"+f.Name)
			}
		})
		if len(bad) > 0 {
			return nil, errs.New(errs.BadInput, strings.Join(bad, ", ")+" cannot be used with --from", "put url, title and author on the batch line: <file>|<url>|<title>|<author>")
		}
	} else if len(pos) != 1 {
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
	arg := ""
	if !batch {
		arg = pos[0]
	}
	o := pipeline.Options{
		Arg: arg, Stdin: env.Stdin, Medium: *as, URL: *url, Title: *title, Author: *author,
		Note: *note, Hunch: *hunch, Force: *force, DryRun: env.DryRun, NoCommit: env.NoCommit,
		PDFCommand: cfg.PDF.Command,
	}
	if batch {
		res, err := pipeline.AddBatch(context.Background(), d, *from, o)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	rep, err := pipeline.Add(context.Background(), d, o)
	if err != nil {
		return nil, err
	}
	return rep, nil
}
