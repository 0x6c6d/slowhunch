package cli

import (
	"encoding/json"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func init() {
	register("get", "print one entry by id", cmdGet)
}

type getResult struct {
	ID, Type, Path, Rev string
	Fields              map[string]any
	Body                string
	Backlinks           []string
	raw                 []byte
}

func (g getResult) Text() string { return string(g.raw) }

func (g getResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"id": g.ID, "type": g.Type, "path": g.Path, "rev": g.Rev,
		"fields": g.Fields, "body": g.Body, "backlinks": g.Backlinks})
}

func cmdGet(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("get"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "get needs exactly one id", "usage: slowhunch get <id>")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	e, ok := ix.Get(pos[0])
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+pos[0], "try slowhunch search")
	}
	bl := ix.Backlinks(e.ID)
	if bl == nil {
		bl = []string{}
	}
	return getResult{ID: e.ID, Type: e.Type, Path: e.Path, Rev: frontmatter.Rev(e.Doc),
		Fields: docToJSON(e.Doc), Body: e.Doc.Body, Backlinks: bl, raw: e.Raw}, nil
}
