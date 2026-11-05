package search

import "math"

// Ranked is one ranked search hit.
type Ranked struct {
	ID      string
	Score   float64
	Quote   string
	Locator string
	Source  string
}

// Score combines cosine similarity with a mild recency boost and a hop penalty.
func Score(cosine float64, daysOld int, hops int) float64 {
	if daysOld < 0 {
		daysOld = 0
	}
	if hops < 0 {
		hops = 0
	}
	recency := 1 / (1 + float64(daysOld)/365)
	return cosine * (1 + 0.1*recency) / float64(1+hops)
}

// Tokens estimates the token cost of a hit: ceil(bytes/4).
func (r Ranked) Tokens() int {
	return (len(r.Quote) + len(r.Locator) + len(r.Source) + 3) / 4
}

// Budget keeps hits in order while the running token total stays within
// budget (0 = unlimited), stopping at the first that would exceed it.
func Budget(hits []Ranked, budget int) ([]Ranked, int) {
	total := 0
	for i, h := range hits {
		t := h.Tokens()
		if budget > 0 && total+t > budget {
			return hits[:i], total
		}
		total += t
	}
	return hits, total
}

// Round4 rounds to 4 decimals.
func Round4(f float64) float64 { return math.Round(f*10000) / 10000 }
