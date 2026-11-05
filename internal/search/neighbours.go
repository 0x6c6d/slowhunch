package search

// Neighbours returns the top near hits for text plus the most similar
// document of the other class (aesthetic(id) != newIsAesthetic) that is not
// among them; far is nil if there is none.
func Neighbours(ix *Index, text string, near int, aesthetic func(id string) bool, newIsAesthetic bool) (nearHits []Hit, far *Hit) {
	nearHits = ix.Similar(text, near, nil)
	in := map[string]bool{}
	for _, h := range nearHits {
		in[h.ID] = true
	}
	q := ix.Query(text)
	for _, id := range ix.ids {
		if in[id] || aesthetic(id) == newIsAesthetic {
			continue
		}
		s := Cosine(q, ix.vecs[id])
		if far == nil || s > far.Score || (s == far.Score && id < far.ID) {
			far = &Hit{id, s}
		}
	}
	return nearHits, far
}
