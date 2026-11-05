package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("init", "create a vault in <dir>", cmdInit)
}

const agentText = `# Agent instructions for this slowhunch vault

1. Run ` + "`slowhunch skill`" + ` at the start of every session.
2. Run ` + "`slowhunch verify`" + ` before and after any batch of changes.
3. Write only through the ` + "`slowhunch`" + ` CLI; never edit vault files directly.
`

const gitignoreText = ".slowhunch/cache/\n.slowhunch/audit.log\n.slowhunch/state.json\n.slowhunch/reports/\n"

func cmdInit(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("init"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) < 1 {
		return nil, errs.New(errs.BadInput, "init needs a directory", "usage: slowhunch init <dir>")
	}
	if err := vault.GuardPath(pos[0]); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(pos[0])
	if err != nil {
		return nil, errs.New(errs.BadInput, err.Error(), "")
	}
	if err := vault.GuardPath(abs); err != nil {
		return nil, err
	}
	warnings := []map[string]string{}
	gitOK := gitops.Available()
	if !gitOK {
		warnings = append(warnings, map[string]string{"code": errs.GitMissing, "path": "",
			"message": "git not found; vault created without commits"})
	}

	files := map[string][]byte{}
	cfgRel := ".slowhunch/config.json"
	files[cfgRel] = vault.MarshalConfig(vault.DefaultConfig())
	for role, b := range provider.DefaultPrompts() {
		files[".slowhunch/prompts/"+role+".md"] = b
	}
	files[".gitignore"] = []byte(gitignoreText)
	files["AGENTS.md"] = []byte(agentText)
	files["CLAUDE.md"] = []byte(agentText)
	var todo []string
	for rel := range files {
		if _, err := os.Lstat(filepath.Join(abs, filepath.FromSlash(rel))); err == nil {
			if rel == "AGENTS.md" || rel == "CLAUDE.md" {
				warnings = append(warnings, map[string]string{"code": errs.Exists, "path": rel, "message": "left unchanged"})
			}
			continue
		}
		todo = append(todo, rel)
	}
	sort.Strings(todo)
	sort.Slice(warnings, func(i, j int) bool { return warnings[i]["path"] < warnings[j]["path"] })

	if env.DryRun {
		return map[string]any{"vault": abs, "created": append([]string{}, todo...), "warnings": warnings,
			"git": map[string]any{"commit": ""}}, nil
	}

	for _, f := range []string{"", "notes", "hunches", "sources", "topics", "proposals", "inbox", "archive", ".slowhunch", ".slowhunch/prompts"} {
		if err := os.MkdirAll(filepath.Join(abs, filepath.FromSlash(f)), 0o755); err != nil {
			return nil, errs.Fatal(errs.BadInput, err.Error(), "")
		}
	}
	if gitOK && !env.NoCommit && !gitops.IsRepo(abs) {
		if err := gitops.Init(abs); err != nil {
			return nil, err
		}
	}
	v, err := vault.Open(abs)
	if err != nil {
		return nil, err
	}
	cfg := vault.DefaultConfig()
	if _, err := os.Lstat(vault.ConfigPath(v.Root)); err == nil {
		if cfg, err = vault.LoadConfig(v.Root); err != nil {
			return nil, err
		}
	}
	pr, err := provider.LoadPrompts(v.Root)
	if err != nil {
		return nil, err
	}
	if todo != nil {
		st, err := v.LoadState()
		if err != nil {
			return nil, err
		}
		st.PromptVersions = provider.Versions(pr)
		if err := v.SaveState(st); err != nil {
			return nil, err
		}
	}
	tx, err := env.begin(v, cfg, "init")
	if err != nil {
		return nil, err
	}
	created := []string{}
	for _, rel := range todo {
		if err := tx.WriteFile(rel, files[rel]); err != nil {
			return nil, err
		}
		created = append(created, rel)
	}
	hash, err := tx.Commit([]string{filepath.Base(abs)}, "files: "+strconv.Itoa(len(created)))
	if err != nil {
		return nil, err
	}
	return map[string]any{"vault": abs, "created": created, "warnings": warnings,
		"git": map[string]any{"commit": hash}}, nil
}
