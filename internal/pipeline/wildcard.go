package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"

	"github.com/0x6c6d/slowhunch/internal/report"
)

// wildcardTag is the tag every atom of a kept wildcard carries.
const wildcardTag = "wildcard"

// chooseWildcards returns, per pool item, whether the wildcard lane keeps it:
// k = floor(share * len(pool)) items with the lowest sha256(text_hash) hex,
// ties broken by pool (entry) order.
func chooseWildcards(share float64, pool []*itemRun) []bool {
	kept := make([]bool, len(pool))
	k := int(math.Floor(share * float64(len(pool))))
	if k <= 0 {
		return kept
	}
	keys := make([]string, len(pool))
	order := make([]int, len(pool))
	for i, r := range pool {
		sum := sha256.Sum256([]byte(r.textHash))
		keys[i] = hex.EncodeToString(sum[:])
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return keys[order[a]] < keys[order[b]] })
	for _, i := range order[:min(k, len(order))] {
		kept[i] = true
	}
	return kept
}

// recordWildcard lists the rejected item r in its report's wildcards under the
// name item; a kept item is reopened so it continues through extract, place
// and write.
func recordWildcard(r *itemRun, item string, kept bool) {
	rep := r.rep
	reason := ""
	if n := len(rep.Errors); n > 0 {
		reason = rep.Errors[n-1].Reason
	}
	rep.Wildcards = append(rep.Wildcards, report.Wildcard{Item: item, Reason: reason, Kept: kept})
	rep.Counts["wildcards"] = len(rep.Wildcards)
	rep.Counts["wildcards_kept"] += 0 // present even when nothing is kept
	if kept {
		rep.Counts["wildcards_kept"]++
		// Drop the gate's REJECTED error: the item goes on.
		rep.Errors = rep.Errors[:len(rep.Errors)-1]
		rep.Counts["errors"] = len(rep.Errors)
		rep.Source = report.Link(r.it.SourceID)
		rep.Git = report.GitInfo{}
		r.done = false
		r.wildcard = true
	}
}

// addTag appends tag to tags unless it is already there.
func addTag(tags []string, tag string) []string {
	for _, t := range tags {
		if t == tag {
			return tags
		}
	}
	return append(tags, tag)
}
