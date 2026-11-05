// Package search is a small in-memory TF-IDF index with cosine similarity.
package search

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Doc is one document to index.
type Doc struct{ ID, Text string }

// Vector is an L2-normalized sparse TF-IDF vector.
type Vector map[string]float64

// Hit is a scored document.
type Hit struct {
	ID    string
	Score float64
}

// Index is an immutable TF-IDF index.
type Index struct {
	n    int
	idf  map[string]float64
	vecs map[string]Vector
	ids  []string
}

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields("the a an and or of to in is it that this for on with as are be by at from") {
		stop[w] = true
	}
}

// Tokenize lowercases s and splits on non letters/digits, dropping
// single-character tokens and common English stopwords.
func Tokenize(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(f))
	for _, t := range f {
		if len([]rune(t)) < 2 || stop[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func counts(text string) (map[string]int, int) {
	toks := Tokenize(text)
	c := make(map[string]int, len(toks))
	for _, t := range toks {
		c[t]++
	}
	return c, len(toks)
}

func normalize(v Vector) Vector {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return Vector{}
	}
	n := math.Sqrt(sum)
	for k := range v {
		v[k] /= n
	}
	return v
}

// Build indexes docs. A repeated ID keeps the last text.
func Build(docs []Doc) *Index {
	ix := &Index{idf: map[string]float64{}, vecs: map[string]Vector{}}
	type tc struct {
		c map[string]int
		n int
	}
	byID := map[string]tc{}
	for _, d := range docs {
		c, n := counts(d.Text)
		if _, ok := byID[d.ID]; !ok {
			ix.ids = append(ix.ids, d.ID)
		}
		byID[d.ID] = tc{c, n}
	}
	ix.n = len(ix.ids)
	df := map[string]int{}
	for _, id := range ix.ids {
		for t := range byID[id].c {
			df[t]++
		}
	}
	for t, d := range df {
		ix.idf[t] = math.Log(float64(1+ix.n)/float64(1+d)) + 1
	}
	for _, id := range ix.ids {
		ix.vecs[id] = ix.vector(byID[id].c, byID[id].n)
	}
	return ix
}

func (ix *Index) vector(c map[string]int, n int) Vector {
	v := Vector{}
	if n == 0 {
		return v
	}
	for t, k := range c {
		idf, ok := ix.idf[t]
		if !ok {
			idf = math.Log(float64(1+ix.n)) + 1 // df = 0
		}
		v[t] = float64(k) / float64(n) * idf
	}
	return normalize(v)
}

// Len returns the number of indexed documents.
func (ix *Index) Len() int { return ix.n }

// Query returns the TF-IDF vector of text under this index's IDF.
func (ix *Index) Query(text string) Vector {
	c, n := counts(text)
	return ix.vector(c, n)
}

// DocVector returns the vector of a document, or nil if unknown.
func (ix *Index) DocVector(id string) Vector {
	v, ok := ix.vecs[id]
	if !ok {
		return nil
	}
	return v
}

// Cosine returns the dot product of two normalized vectors (0 if empty).
func Cosine(a, b Vector) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	var s float64
	for k, x := range a {
		s += x * b[k]
	}
	return s
}

func sortHits(h []Hit) {
	sort.Slice(h, func(i, j int) bool {
		if h[i].Score != h[j].Score {
			return h[i].Score > h[j].Score
		}
		return h[i].ID < h[j].ID
	})
}

// Similar returns documents with score > 0, best first, then by id.
func (ix *Index) Similar(text string, limit int, keep func(id string) bool) []Hit {
	q := ix.Query(text)
	var hits []Hit
	for _, id := range ix.ids {
		if keep != nil && !keep(id) {
			continue
		}
		if s := Cosine(q, ix.vecs[id]); s > 0 {
			hits = append(hits, Hit{id, s})
		}
	}
	sortHits(hits)
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
