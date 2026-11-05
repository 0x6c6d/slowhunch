package cli

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("init", "create a vault in <dir>", cmdInit)
}

func cmdInit(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("init"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) < 1 {
		return nil, errs.New(errs.BadInput, "init needs a directory", "usage: slowhunch init <dir>")
	}
	abs, err := filepath.Abs(pos[0])
	if err != nil {
		return nil, errs.New(errs.BadInput, err.Error(), "")
	}
	created := []string{}
	if _, err := os.Stat(abs); err != nil {
		created = append(created, ".")
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, errs.Fatal(errs.BadInput, err.Error(), "")
	}
	for _, f := range []string{"notes", "hunches", "sources", "topics", "proposals", "inbox", "archive", ".slowhunch"} {
		p := filepath.Join(abs, f)
		if _, err := os.Stat(p); err != nil {
			created = append(created, f)
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, errs.Fatal(errs.BadInput, err.Error(), "")
		}
	}
	cp := vault.ConfigPath(abs)
	if _, err := os.Stat(cp); err != nil {
		if err := os.WriteFile(cp, vault.MarshalConfig(vault.DefaultConfig()), 0o644); err != nil {
			return nil, errs.Fatal(errs.BadInput, err.Error(), "")
		}
		created = append(created, filepath.Join(".slowhunch", "config.json"))
	}
	sort.Strings(created)
	return map[string]any{"vault": abs, "created": created}, nil
}
