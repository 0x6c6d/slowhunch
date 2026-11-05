package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/report"
)

func init() {
	register("stats", "show counts, ratios and token usage (read-only)", cmdStats)
}

type statsResult struct{ report.Stats }

func (r statsResult) MarshalJSON() ([]byte, error) { return json.Marshal(r.Stats) }

func (r statsResult) Text() string {
	var b strings.Builder
	row := func(k string, v any) { fmt.Fprintf(&b, "%-22s %v\n", k, v) }
	group := func(prefix string, m map[string]int) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			row("  "+prefix+" "+k, m[k])
		}
	}
	row("atoms", r.Atoms)
	group("medium", r.AtomsByMedium)
	group("stance", r.AtomsByStance)
	if r.AssertsReadsRatio == nil {
		row("asserts/reads", "n/a")
	} else {
		row("asserts/reads", *r.AssertsReadsRatio)
	}
	row("hunches", r.Hunches)
	group("state", r.HunchesByState)
	row("contradictions", r.Contradictions)
	row("echoes", r.Echoes)
	roles := make([]string, 0, len(r.TokensByRole))
	for k := range r.TokensByRole {
		roles = append(roles, k)
	}
	sort.Strings(roles)
	for _, k := range roles {
		u := r.TokensByRole[k]
		row("  tokens "+k, fmt.Sprintf("in=%d out=%d", u.TokensIn, u.TokensOut))
	}
	return strings.TrimRight(b.String(), "\n")
}

func cmdStats(env *Env, args []string) (any, error) {
	fs := newFlagSet("stats")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "stats takes no arguments", "usage: slowhunch stats")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	reps, err := report.LoadAll(v)
	if err != nil {
		return nil, err
	}
	return statsResult{report.Compute(ix, reps)}, nil
}
