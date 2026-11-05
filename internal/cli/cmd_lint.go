package cli

import (
	"encoding/json"
	"fmt"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("lint", "check the vault against the note rules", cmdLint)
}

type lintResult struct{ Checked int }

func (l lintResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"failures": []any{}, "checked": l.Checked})
}

func (l lintResult) Text() string { return fmt.Sprintf("ok: %d notes checked", l.Checked) }

func cmdLint(env *Env, args []string) (any, error) {
	fs := newFlagSet("lint")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "lint takes no arguments", "usage: slowhunch lint")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, lerrs, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	ps := vault.Lint(ix, lerrs)
	if len(ps) == 0 {
		return lintResult{len(ix.OfType("note"))}, nil
	}
	if !env.JSON {
		for _, p := range ps {
			fmt.Fprintf(env.Stdout, "%s:%d: %s %s\n", p.Path, p.Line, p.Code, p.Message)
		}
	}
	e := errs.New(errs.LintFailed, fmt.Sprintf("%d lint failures", len(ps)), "fix the listed files")
	e.Extra = map[string]any{"failures": ps}
	return nil, e
}
