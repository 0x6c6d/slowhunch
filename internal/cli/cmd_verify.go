package cli

import (
	"encoding/json"
	"fmt"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("verify", "re-derive the index from files and report inconsistencies (read-only)", cmdVerify)
}

type verifyResult struct{ Checked int }

func (r verifyResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"problems": []any{}, "checked": r.Checked})
}

func (r verifyResult) Text() string { return fmt.Sprintf("ok: %d files verified", r.Checked) }

func cmdVerify(env *Env, args []string) (any, error) {
	fs := newFlagSet("verify")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "verify takes no arguments", "usage: slowhunch verify")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ps, n, err := vault.Verify(v)
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return verifyResult{n}, nil
	}
	if !env.JSON {
		for _, p := range ps {
			fmt.Fprintf(env.Stdout, "%s:%d: %s %s %s\n", p.Path, p.Line, p.Code, p.ID, p.Message)
		}
	}
	e := errs.New(errs.LintFailed, fmt.Sprintf("%d verify problems", len(ps)), "inspect the listed ids")
	e.Extra = map[string]any{"problems": ps}
	return nil, e
}
