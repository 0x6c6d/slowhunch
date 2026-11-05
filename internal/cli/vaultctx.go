package cli

import (
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// openVault resolves the vault root, opens it and loads its config.
func (e *Env) openVault() (*vault.Vault, vault.Config, error) {
	root, err := e.VaultRoot()
	if err != nil {
		return nil, vault.Config{}, err
	}
	v, err := vault.Open(root)
	if err != nil {
		return nil, vault.Config{}, err
	}
	cfg, err := vault.LoadConfig(v.Root)
	if err != nil {
		return nil, vault.Config{}, err
	}
	return v, cfg, nil
}

// begin starts a transaction honouring --dry-run, --no-commit, config and git availability.
func (e *Env) begin(v *vault.Vault, cfg vault.Config, command string) (*vault.Txn, error) {
	o := vault.TxnOptions{DryRun: e.DryRun, Now: e.Now}
	if e.NoCommit || !cfg.Git.Commit || !gitops.Available() {
		o.NoCommit = true
	} else {
		o.Committer = gitops.New(v.Root)
	}
	pr, err := provider.LoadPrompts(v.Root)
	if err != nil {
		return nil, err
	}
	o.PromptVersions = provider.Versions(pr)
	return v.Begin(command, o)
}

func (e *Env) today() string { return e.Now().Format("2006-01-02") }
