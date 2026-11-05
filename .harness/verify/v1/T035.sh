#!/usr/bin/env bash
# Verification for T035 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "place tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestPlace' TestPlaceDuplicateLowCosine TestPlaceDuplicateHigh TestPlaceReads TestPlaceEcho TestPlaceContradictNoTag TestPlaceContradictSharedTag TestPlaceBadTarget TestPlaceExcerptCap TestPlaceOneCallPerAtom TestPlaceNoNeighbours
CMD
expect_ok "probe: spec threshold table (0.5, 0.9, 0.95, 0.4, 0.1)" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	cfg := vault.DefaultConfig().Place
	nb := func(c float64) []pipeline.Neighbour { return []pipeline.Neighbour{{ID: "t", Cosine: c}} }
	as := pipeline.Atom{Stance: "asserts"}
	rd := pipeline.Atom{Stance: "reads"}
	cr := pipeline.Atom{Stance: "crafts"}
	ok := pipeline.Decide(as, false, pipeline.Verdict{Verdict: "duplicate", Target: "t"}, nb(0.5), cfg).Action == "extends" &&
		pipeline.Decide(as, false, pipeline.Verdict{Verdict: "duplicate", Target: "t"}, nb(0.9), cfg).Action == "duplicate" &&
		pipeline.Decide(rd, true, pipeline.Verdict{Verdict: "duplicate", Target: "t"}, nb(0.95), cfg).Action == "rereads" &&
		pipeline.Decide(cr, true, pipeline.Verdict{Verdict: "new"}, nb(0.4), cfg).Echo &&
		pipeline.Decide(as, false, pipeline.Verdict{Verdict: "contradicts", Target: "t"}, nb(0.1), cfg).Action == "extends"
	if !ok {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: a target outside the neighbour list is never acted on" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	cfg := vault.DefaultConfig().Place
	nbs := []pipeline.Neighbour{{ID: "t", Cosine: 0.99}}
	for _, target := range []string{"../../etc/passwd", "ghost", "", "T"} {
		for _, v := range []string{"duplicate", "extends", "contradicts"} {
			d := pipeline.Decide(pipeline.Atom{Stance: "asserts"}, false, pipeline.Verdict{Verdict: v, Target: target}, nbs, cfg)
			if d.Action != "new" || d.Target != "" {
				os.Exit(0)
			}
		}
	}
	os.Exit(1)
}
GO
CMD
finish
