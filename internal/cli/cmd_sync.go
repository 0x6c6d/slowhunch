package cli

import (
	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
)

func init() {
	register("sync", "pull then push the vault's git remote", cmdSync)
}

type syncResult struct {
	Pulled   int          `json:"pulled"`
	Pushed   bool         `json:"pushed"`
	Behind   *int         `json:"behind,omitempty"`
	Warnings []runWarning `json:"warnings,omitempty"`
}

// cmdSync pulls (rebase, autostash; skipped without an upstream or with
// --no-pull) and then pushes git.branch (or the current branch) to
// git.remote whatever git.push says (skipped with --no-push). It writes no
// vault file and makes no commit. With --dry-run it only reports git.behind.
func cmdSync(env *Env, args []string) (any, error) {
	fs := newFlagSet("sync")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "sync takes no arguments", "usage: slowhunch sync [--no-pull] [--no-push]")
	}
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	noRemote := errs.New(errs.NotFound, "the vault has no git remote "+cfg.Git.Remote, "git remote add origin <url>")
	if !gitops.Available() || !gitops.IsRepo(v.Root) {
		return nil, noRemote
	}
	repo := gitops.New(v.Root)
	if ok, err := repo.HasRemote(cfg.Git.Remote); err != nil || !ok {
		return nil, noRemote
	}
	if env.DryRun {
		n := dryBehind(v)
		res := syncResult{Behind: &n}
		if n > 0 {
			res.Warnings = []runWarning{behindWarning(n)}
		}
		return res, nil
	}
	res := syncResult{}
	if !env.NoPull {
		if ok, err := repo.HasUpstream(); err == nil && ok {
			n, err := repo.PullRebase()
			if err != nil {
				return nil, err
			}
			res.Pulled = n
		}
	}
	if !env.NoPush {
		branch := cfg.Git.Branch
		if branch == "" {
			if branch, err = repo.CurrentBranch(); err != nil {
				return nil, errs.New(errs.PushFailed, "cannot tell the current branch", "set git.branch or check out a branch")
			}
		}
		if err := repo.Push(cfg.Git.Remote, branch); err != nil {
			return nil, err
		}
		res.Pushed = true
	}
	return res, nil
}
