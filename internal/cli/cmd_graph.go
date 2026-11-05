package cli

import (
	"github.com/0x6c6d/slowhunch/internal/errs"
)

func init() {
	register("backlinks", "list entries linking to <id>", cmdBacklinks)
	register("graph", "show the link neighbourhood of <id> (-depth N, 1-5)", cmdGraph)
}

func cmdBacklinks(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("backlinks"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "backlinks needs exactly one id", "usage: slowhunch backlinks <id>")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	if _, ok := ix.Get(pos[0]); !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+pos[0], "try slowhunch search")
	}
	bl := ix.Backlinks(pos[0])
	if bl == nil {
		bl = []string{}
	}
	return map[string]any{"id": pos[0], "backlinks": bl}, nil
}

func cmdGraph(env *Env, args []string) (any, error) {
	fs := newFlagSet("graph")
	depth := fs.Int("depth", 1, "how many link hops to follow (1-5)")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "graph needs exactly one id", "usage: slowhunch graph <id> [--depth N]")
	}
	if *depth < 1 || *depth > 5 {
		return nil, badInput("depth must be between 1 and 5")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	if _, ok := ix.Get(pos[0]); !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+pos[0], "try slowhunch search")
	}
	nodes, es := ix.Neighbourhood(pos[0], *depth)
	edges := make([]map[string]string, 0, len(es))
	for _, e := range es {
		edges = append(edges, map[string]string{"from": e[0], "to": e[1]})
	}
	return map[string]any{"id": pos[0], "depth": *depth, "nodes": nodes, "edges": edges}, nil
}
