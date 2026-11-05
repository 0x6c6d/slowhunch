package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// runWarning is a non-fatal problem of a vault-changing run.
type runWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

// runPrep is the outcome of prepareRun: commits pulled and warnings, and
// the push planned for after the command (see finishRun).
type runPrep struct {
	Pulled   int
	Warnings []runWarning
	Pushed   bool
	Behind   int // dry run: upstream commits HEAD lacks (Repo.Behind)

	dry      bool         // a dry run: no pull, no push, git.behind reported
	readOnly bool         // a read-only command run with --sync
	push     bool         // a push was asked for and is allowed
	vault    *vault.Vault // the vault of the run
	cfg      vault.Config
	head     string      // HEAD when the run started
	pushWarn *runWarning // PUSH_FAILED of this run
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

func (p *runPrep) behind() int {
	if p == nil {
		return 0
	}
	return p.Behind
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
	return e.prepare(v, cfg, e.DryRun)
}

// prepare is prepareRun with the dry-run decision given by the caller (a
// command may have its own dry-run flag). A dry run never pulls or pushes:
// it records Repo.Behind (from the already-fetched upstream ref, no network)
// and, when behind, a BEHIND warning.
func (e *Env) prepare(v *vault.Vault, cfg vault.Config, dry bool) (*runPrep, error) {
	p := &runPrep{dry: dry}
	e.prep = p
	if dry {
		p.Behind = dryBehind(v)
		if p.Behind > 0 {
			p.Warnings = append(p.Warnings, behindWarning(p.Behind))
		}
		return p, nil
	}
	// Plan the push after the resume commit and the pull, so HEAD then marks
	// the start of this run's own changes.
	defer e.planPush(p, v, cfg)
	if e.NoPull || e.NoCommit || !cfg.Git.Commit || !cfg.Git.Pull || !gitops.Available() {
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

// dryBehind returns Repo.Behind for the vault, 0 without git, a repository
// or an upstream.
func dryBehind(v *vault.Vault) int {
	if !gitops.Available() || !gitops.IsRepo(v.Root) {
		return 0
	}
	n, err := gitops.New(v.Root).Behind()
	if err != nil {
		return 0
	}
	return n
}

func behindWarning(n int) runWarning {
	return runWarning{Code: errs.Behind, Message: fmt.Sprintf("the vault is %d commit(s) behind its upstream", n),
		Hint: "run slowhunch sync to pull them"}
}

// readOnlyCommands never pull unless --sync is given.
var readOnlyCommands = map[string]bool{
	"search": true, "get": true, "list": true, "stats": true, "review": true, "verify": true, "lint": true,
}

// syncRead runs before a read-only command given --sync: it pulls with the
// rules and outcomes of a vault-changing command's pull and never pushes.
// When the vault cannot be opened it does nothing and the command reports
// the error.
func (e *Env) syncRead() error {
	v, cfg, err := e.openVault()
	if err != nil {
		return nil
	}
	p, err := e.prepareRun(v, cfg)
	if err != nil {
		return err
	}
	p.push = false
	p.readOnly = true
	return nil
}

// syncedResult is a read-only result with pulled (and any warnings) added
// to its JSON object.
type syncedResult struct {
	res      any
	pulled   int
	warnings []runWarning
}

func (s syncedResult) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(s.res)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return raw, nil
	}
	extra := fmt.Sprintf(`"pulled":%d`, s.pulled)
	if len(s.warnings) > 0 {
		w, err := json.Marshal(s.warnings)
		if err != nil {
			return nil, err
		}
		extra += `,"warnings":` + string(w)
	}
	inner := bytes.TrimSpace(raw[1 : len(raw)-1])
	if len(inner) == 0 {
		return []byte("{" + extra + "}"), nil
	}
	return []byte("{" + string(inner) + "," + extra + "}"), nil
}

// planPush decides, before anything is written, whether the run pushes:
// (git.push or --push) and not --no-push, not --dry-run, commits enabled,
// git available and the vault a repository. It records HEAD so finishRun
// can tell whether the run made a commit.
func (e *Env) planPush(p *runPrep, v *vault.Vault, cfg vault.Config) {
	if !(cfg.Git.Push || e.Push) || e.NoPush || e.DryRun || e.NoCommit || !cfg.Git.Commit || !gitops.Available() {
		return
	}
	if !gitops.IsRepo(v.Root) {
		return
	}
	head, err := gitops.New(v.Root).Head()
	if err != nil {
		head = ""
	}
	p.push, p.vault, p.cfg, p.head = true, v, cfg, head
}

// runPush pushes git.branch (or the current branch) to git.remote when the
// run made a commit, or when the branch is ahead of its upstream (commits of
// an earlier run whose push failed). Without the remote it does nothing. A
// failed push becomes the PUSH_FAILED warning; the commit is kept, the push
// is never retried within the run and never forced.
func (p *runPrep) runPush() {
	if p == nil || !p.push {
		return
	}
	repo := gitops.New(p.vault.Root)
	if ok, err := repo.HasRemote(p.cfg.Git.Remote); err != nil || !ok {
		return
	}
	head, err := repo.Head()
	if err != nil {
		return
	}
	if head == p.head {
		n, err := repo.Ahead()
		if err != nil || n == 0 {
			return
		}
	}
	branch := p.cfg.Git.Branch
	if branch == "" {
		if branch, err = repo.CurrentBranch(); err != nil {
			p.warnPush(errs.New(errs.PushFailed, "cannot tell the current branch", "set git.branch or check out a branch"))
			return
		}
	}
	if err := repo.Push(p.cfg.Git.Remote, branch); err != nil {
		p.warnPush(err)
		return
	}
	p.Pushed = true
}

func (p *runPrep) warnPush(err error) {
	w := runWarning{Code: errs.PushFailed, Message: err.Error(),
		Hint: "the commit is kept; the next run that pushes sends it"}
	if ee, ok := err.(*errs.Error); ok {
		w.Message = ee.Message
		if ee.Hint != "" {
			w.Hint = ee.Hint
		}
	}
	p.pushWarn = &w
	p.Warnings = append(p.Warnings, w)
}

// finishRun runs after a vault-changing command succeeded: it pushes (see
// runPush) and records the outcome in the result. Reports of add and collide
// get git.pushed and a PUSH_FAILED entry in errors, and are saved again when
// the command saved them; map results get git.pulled, git.pushed and the
// warnings array.
func (e *Env) finishRun(res any) (any, error) {
	p := e.prep
	if p == nil {
		return res, nil
	}
	if p.readOnly {
		if !e.JSON {
			return res, nil
		}
		return syncedResult{res: res, pulled: p.Pulled, warnings: p.Warnings}, nil
	}
	p.runPush()
	var pushErr []report.ItemError
	if p.pushWarn != nil {
		pushErr = []report.ItemError{{Item: "git", Code: p.pushWarn.Code, Reason: p.pushWarn.Message}}
	}
	switch r := res.(type) {
	case *report.Report:
		return r, p.patchReport(r, pushErr, r.RunID+".json")
	case collideResult:
		return r, p.patchReport(r.Report, pushErr, r.RunID+".json")
	case *pipeline.BatchResult:
		r.Git.Pushed = p.Pushed
		r.Git.Behind = p.Behind
		r.Errors = append(r.Errors, pushErr...)
		for i, it := range r.Items {
			if err := p.patchReport(it, nil, fmt.Sprintf("%s-%d.json", it.RunID, i+1)); err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	return p.decorate(res), nil
}

// patchReport sets git.pushed and appends extra to rep, then rewrites the
// saved report .slowhunch/reports/<name> if the command saved one. The
// reports folder is git-ignored, so the rewrite needs no commit.
func (p *runPrep) patchReport(rep *report.Report, extra []report.ItemError, name string) error {
	if rep == nil {
		return nil
	}
	rep.Git.Pushed = p.Pushed
	rep.Git.Behind = p.Behind
	rep.Errors = append(rep.Errors, extra...)
	if p.vault == nil || (!p.Pushed && len(extra) == 0) {
		return nil
	}
	rel := ".slowhunch/reports/" + name
	if !p.vault.Exists(rel) {
		return nil
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return p.vault.WriteFile(rel, append(b, '\n'))
}

// decorate adds pulled and pushed to the git object of a map result and the
// warnings as a top-level array.
func (p *runPrep) decorate(res any) any {
	m, ok := res.(map[string]any)
	if p == nil || !ok {
		return res
	}
	if g, ok := m["git"].(map[string]any); ok {
		g["pulled"] = p.Pulled
		g["pushed"] = p.Pushed
		if p.dry {
			g["behind"] = p.Behind
		}
	}
	if len(p.Warnings) > 0 {
		m["warnings"] = p.Warnings
	}
	return m
}
