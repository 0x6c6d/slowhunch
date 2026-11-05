package cli

import (
	"github.com/0x6c6d/slowhunch/internal/errs"
)

func init() {
	register("delete", "archive an entry (soft), or remove it with --hard --yes", cmdDelete)
}

func cmdDelete(env *Env, args []string) (any, error) {
	fs := newFlagSet("delete")
	hard := fs.Bool("hard", false, "remove the file instead of archiving it (needs --yes)")
	yes := fs.Bool("yes", false, "confirm a hard delete")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch delete <id> [--hard --yes]"
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "delete needs exactly one id", usage)
	}
	id := pos[0]
	if *hard && !*yes {
		return nil, errs.New(errs.BadInput, "hard delete needs --yes", usage)
	}

	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	e, ok := ix.Get(id)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+id, "try slowhunch search")
	}

	if *hard {
		txn, err := env.begin(v, cfg, "delete")
		if err != nil {
			return nil, err
		}
		if err := txn.Remove(e.Path); err != nil {
			return nil, err
		}
		hash, err := txn.Commit([]string{id}, "removed: 1")
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "removed": true, "path": e.Path, "git": map[string]any{"commit": hash}}, nil
	}

	target := "archive/" + id + ".md"
	if v.Exists(target) {
		return nil, errs.New(errs.Exists, "target exists: "+target, "rename or remove the archived file first")
	}
	txn, err := env.begin(v, cfg, "delete")
	if err != nil {
		return nil, err
	}
	if err := txn.Move(e.Path, target); err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{id}, "archived: 1")
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "archived": target, "path": e.Path, "git": map[string]any{"commit": hash}}, nil
}
