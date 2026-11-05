package cli

import (
	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// runWarning is a non-fatal problem of a vault-changing run.
type runWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

// runPrep is the outcome of prepareRun: commits pulled and warnings.
type runPrep struct {
	Pulled   int
	Warnings []runWarning
}

// itemErrors returns the warnings in the errors form of a report.
func (p *runPrep) itemErrors() []report.ItemError {
	if p == nil {
		return nil
	}
	out := make([]report.ItemError, 0, len(p.Warnings))
	for _, w := range p.Warnings {
		out = append(out, report.ItemError{Item: "git", Code: w.Code, Reason: w.Message})
	}
	return out
}

func (p *runPrep) pulled() int {
	if p == nil {
		return 0
	}
	return p.Pulled
}

// openRun opens the vault for a vault-changing command and runs prepareRun.
func (e *Env) openRun() (*vault.Vault, vault.Config, error) {
	v, cfg, err := e.openVault()
	if err != nil {
		return nil, vault.Config{}, err
	}
	if _, err := e.prepareRun(v, cfg); err != nil {
		return nil, vault.Config{}, err
	}
	return v, cfg, nil
}

// prepareRun runs before a vault-changing command loads its index or calls a
// provider: it commits pending files of a crashed run, then pulls (rebase,
// autostash) when git, commits, the remote git.remote, an upstream and
// git.pull are all present and neither --no-pull nor --dry-run is given.
// A pull conflict fails CONFLICT; PULL_FAILED fails only with
// git.require_pull, otherwise it becomes a warning.
func (e *Env) prepareRun(v *vault.Vault, cfg vault.Config) (*runPrep, error) {
	p := &runPrep{}
	e.prep = p
	if e.DryRun || e.NoPull || e.NoCommit || !cfg.Git.Commit || !cfg.Git.Pull || !gitops.Available() {
		return p, nil
	}
	if !gitops.IsRepo(v.Root) {
		return p, nil
	}
	repo := gitops.New(v.Root)
	if ok, err := repo.HasRemote(cfg.Git.Remote); err != nil || !ok {
		return p, nil
	}
	if ok, err := repo.HasUpstream(); err != nil || !ok {
		return p, nil
	}
	// 1. Resume: commit files a crashed run left pending.
	st, err := v.LoadState()
	if err != nil {
		return nil, err
	}
	if len(st.Pending) > 0 {
		txn, err := e.begin(v, cfg, "resume")
		if err != nil {
			return nil, err
		}
		if _, err := txn.Commit(nil, ""); err != nil {
			return nil, err
		}
	}
	// 2. Dirty checks stay per written path (Txn.Touch), as in v1.
	// 3. Pull.
	n, err := repo.PullRebase()
	if err != nil {
		if errs.CodeOf(err) == errs.PullFailed && !cfg.Git.RequirePull {
			msg, hint := err.Error(), ""
			if ee, ok := err.(*errs.Error); ok {
				msg, hint = ee.Message, ee.Hint
			}
			p.Warnings = append(p.Warnings, runWarning{Code: errs.PullFailed, Message: msg, Hint: hint})
			return p, nil
		}
		return nil, err
	}
	p.Pulled = n
	return p, nil
}

// decorate adds pulled to the git object of a map result and the warnings
// as a top-level array.
func (p *runPrep) decorate(res any) any {
	m, ok := res.(map[string]any)
	if p == nil || !ok {
		return res
	}
	if g, ok := m["git"].(map[string]any); ok {
		g["pulled"] = p.Pulled
	}
	if len(p.Warnings) > 0 {
		m["warnings"] = p.Warnings
	}
	return m
}
