#!/usr/bin/env bash
# Verification for T021 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "tf-idf tests" <<'CMD'
gotest "./internal/search/..." 'TestSearchTokenize|TestSearchCosine|TestSearchSimilarOrder|TestSearchNeighbours' TestSearchTokenize TestSearchCosine TestSearchSimilarOrder TestSearchNeighbours
CMD
expect_ok "probe: identical text scores 1, far neighbour from other class" <<'CMD'
goprobe <<'GO'
package main

import (
	"math"
	"os"

	"github.com/0x6c6d/slowhunch/internal/search"
)

func main() {
	docs := []search.Doc{{"a", "trees coming into leaf"}, {"b", "trees leaf green spring"}, {"c", "interest rates inflation bank"}}
	ix := search.Build(docs)
	h := ix.Similar("trees coming into leaf", 5, nil)
	if len(h) < 1 || h[0].ID != "a" || math.Abs(h[0].Score-1) > 1e-9 {
		os.Exit(1)
	}
	aest := func(id string) bool { return id != "c" }
	near, far := search.Neighbours(ix, "trees leaf", 1, aest, true)
	if len(near) != 1 || far == nil || far.ID != "c" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: no far neighbour without another class; disjoint scores 0" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/search"
)

func main() {
	ix := search.Build([]search.Doc{{"a", "trees leaf"}, {"b", "river stone"}})
	_, far := search.Neighbours(ix, "trees", 5, func(string) bool { return true }, true)
	if far != nil || search.Cosine(ix.Query("trees leaf"), ix.Query("river stone")) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
