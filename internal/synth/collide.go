package synth

import (
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/search"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Item is one collision pool member.
type Item struct {
	ID, Type, Medium, Stance, Text string
	Aesthetic                      bool
}

// Pair is a selected collision pair, A < B.
type Pair struct {
	A, B          string
	Cosine, Score float64
	Reserved      bool
}

// Selection is the result of pair selection.
type Selection struct {
	Pairs      []Pair
	Candidates int
	Capped     bool
}

// Pool returns live notes and open or warming hunches, sorted by id.
func Pool(ix *vault.Index) []Item {
	var out []Item
	for _, e := range ix.All() {
		id := e.ID
		var text, stance string
		switch e.Type {
		case "note":
			text = e.Doc.GetString("claim")
			stance = e.Doc.GetString("stance")
		case "hunch":
			st := e.Doc.GetString("state")
			if st != "open" && st != "warming" {
				continue
			}
			text = e.Doc.GetString("question")
		default:
			continue
		}
		m := ix.MediumOf(id)
		out = append(out, Item{ID: id, Type: e.Type, Medium: m, Stance: stance,
			Text: strings.TrimSpace(text + "\n" + e.Doc.Body), Aesthetic: vault.IsAesthetic(m)})
	}
	return out
}

// ExistingPairs returns the pairs already listed in proposals (both orders).
func ExistingPairs(ix *vault.Index) map[[2]string]bool {
	out := map[[2]string]bool{}
	for _, e := range ix.OfType("proposal") {
		items := e.Doc.GetList("items")
		if len(items) < 2 {
			continue
		}
		out[[2]string{items[0], items[1]}] = true
		out[[2]string{items[1], items[0]}] = true
	}
	return out
}

func lessPair(a, b Pair) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.A != b.A {
		return a.A < b.A
	}
	return a.B < b.B
}

// Select picks the collision pairs. It is pure and deterministic.
func Select(items []Item, existing map[[2]string]bool, cfg vault.CollideConfig, limit int) Selection {
	items = append([]Item(nil), items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	docs := make([]search.Doc, len(items))
	for i, it := range items {
		docs[i] = search.Doc{ID: it.ID, Text: it.Text}
	}
	ix := search.Build(docs)
	var cands []Pair
	var reserved []bool
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			a, b := items[i], items[j]
			if a.ID == b.ID || existing[[2]string{a.ID, b.ID}] || existing[[2]string{b.ID, a.ID}] {
				continue
			}
			c := search.Cosine(ix.DocVector(a.ID), ix.DocVector(b.ID))
			if c > cfg.MaxCosine {
				continue
			}
			s := 1 - c
			if a.Aesthetic || b.Aesthetic {
				s *= cfg.AestheticWeight
			}
			cross := a.Medium != b.Medium &&
				((a.Stance == "asserts" && b.Stance == "reads") || (a.Stance == "reads" && b.Stance == "asserts"))
			cands = append(cands, Pair{A: a.ID, B: b.ID, Cosine: c, Score: s})
			reserved = append(reserved, cross)
		}
	}
	idx := make([]int, len(cands))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(x, y int) bool { return lessPair(cands[idx[x]], cands[idx[y]]) })

	n := cfg.MaxPairs
	if limit > 0 && limit < n {
		n = limit
	}
	if n < 0 {
		n = 0
	}
	r := n / 4
	if n >= 4 && r < 1 {
		r = 1
	}
	taken := make([]bool, len(cands))
	var pairs []Pair
	for _, i := range idx {
		if len(pairs) >= r {
			break
		}
		if reserved[i] {
			p := cands[i]
			p.Reserved = true
			pairs = append(pairs, p)
			taken[i] = true
		}
	}
	for _, i := range idx {
		if len(pairs) >= n {
			break
		}
		if !taken[i] {
			pairs = append(pairs, cands[i])
			taken[i] = true
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return lessPair(pairs[i], pairs[j]) })
	return Selection{Pairs: pairs, Candidates: len(cands), Capped: len(cands) > len(pairs)}
}
