package cli

import (
	"slices"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("link", "link entry <from> to <to> (one direction)", cmdLink)
	register("unlink", "remove the link from <from> to <to>", cmdUnlink)
}

func cmdLink(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("link"), args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch link <from> <to>"
	if len(pos) != 2 {
		return nil, errs.New(errs.BadInput, "link needs <from> and <to>", usage)
	}
	from, to := pos[0], pos[1]
	if from == to {
		return nil, badInput("cannot link an entry to itself")
	}
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	fe, ok := ix.Get(from)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+from, "try slowhunch search")
	}
	te, ok := ix.Get(to)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+to, "try slowhunch search")
	}
	field := "links"
	if te.Type == "hunch" || te.Type == "proposal" {
		field = "bears_on"
	}
	result := func(changed bool, hash string) map[string]any {
		return map[string]any{"from": from, "to": to, "field": field, "changed": changed,
			"git": map[string]any{"commit": hash}}
	}
	if slices.Contains(fe.Doc.GetList(field), to) {
		return result(false, ""), nil
	}
	rev := frontmatter.Rev(fe.Doc)
	d := fe.Doc.Clone()
	d.Set(field, append(append([]string{}, fe.Doc.GetList(field)...), to))
	d.Set("updated", env.today())
	if err := vault.Validate(d, from); err != nil {
		return nil, err
	}
	txn, err := env.begin(v, cfg, "link")
	if err != nil {
		return nil, err
	}
	if _, err := txn.PutDoc(fe.Path, d, rev); err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{from}, "linked: 1")
	if err != nil {
		return nil, err
	}
	return result(true, hash), nil
}

func cmdUnlink(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("unlink"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 2 {
		return nil, errs.New(errs.BadInput, "unlink needs <from> and <to>", "usage: slowhunch unlink <from> <to>")
	}
	from, to := pos[0], pos[1]
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	fe, ok := ix.Get(from)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+from, "try slowhunch search")
	}
	result := func(changed bool, hash string) map[string]any {
		return map[string]any{"from": from, "to": to, "changed": changed, "git": map[string]any{"commit": hash}}
	}
	rev := frontmatter.Rev(fe.Doc)
	d := fe.Doc.Clone()
	changed := false
	for _, field := range []string{"bears_on", "links"} {
		l := fe.Doc.GetList(field)
		if !slices.Contains(l, to) {
			continue
		}
		changed = true
		d.Set(field, slices.DeleteFunc(append([]string{}, l...), func(s string) bool { return s == to }))
	}
	if !changed {
		return result(false, ""), nil
	}
	d.Set("updated", env.today())
	if err := vault.Validate(d, from); err != nil {
		return nil, err
	}
	txn, err := env.begin(v, cfg, "unlink")
	if err != nil {
		return nil, err
	}
	if _, err := txn.PutDoc(fe.Path, d, rev); err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{from}, "unlinked: 1")
	if err != nil {
		return nil, err
	}
	return result(true, hash), nil
}
