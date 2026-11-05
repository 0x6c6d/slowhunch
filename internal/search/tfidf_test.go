package search

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSearchTokenize(t *testing.T) {
	got := Tokenize("The Trees, a coming-into LEAF; x 42 café!")
	want := []string{"trees", "coming", "into", "leaf", "42", "café"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSearchCosine(t *testing.T) {
	ix := Build([]Doc{{"a", "trees leaf green"}, {"b", "river stone water"}, {"c", "trees river"}})
	if s := Cosine(ix.Query("trees leaf green"), ix.DocVector("a")); math.Abs(s-1) > 1e-9 {
		t.Fatalf("identical %v", s)
	}
	if s := Cosine(ix.DocVector("a"), ix.DocVector("b")); s != 0 {
		t.Fatalf("disjoint %v", s)
	}
	if s := Cosine(ix.DocVector("a"), ix.DocVector("c")); s <= 0 || s >= 1 {
		t.Fatalf("partial %v", s)
	}
	if Cosine(Vector{}, ix.DocVector("a")) != 0 || ix.DocVector("zz") != nil {
		t.Fatal("empty/unknown")
	}
}

func TestSearchSimilarOrder(t *testing.T) {
	ix := Build([]Doc{{"b", "apple pear"}, {"a", "apple pear"}, {"c", "apple"}, {"d", "other"}})
	h := ix.Similar("apple pear", 0, nil)
	if len(h) != 3 || h[0].ID != "a" || h[1].ID != "b" || h[2].ID != "c" {
		t.Fatalf("%v", h)
	}
	h = ix.Similar("apple pear", 0, func(id string) bool { return id != "a" })
	if len(h) != 2 || h[0].ID != "b" {
		t.Fatalf("%v", h)
	}
	if h = ix.Similar("apple pear", 1, nil); len(h) != 1 || h[0].ID != "a" {
		t.Fatalf("%v", h)
	}
}

func TestSearchNeighbours(t *testing.T) {
	ix := Build([]Doc{{"a", "trees leaf"}, {"b", "trees green"}, {"c", "bank rates"}, {"d", "bank loans"}})
	aest := func(id string) bool { return id == "a" || id == "b" }
	near, far := Neighbours(ix, "trees leaf", 1, aest, true)
	if len(near) != 1 || near[0].ID != "a" {
		t.Fatalf("%v", near)
	}
	if far == nil || (far.ID != "c" && far.ID != "d") || far.Score != 0 {
		t.Fatalf("%v", far)
	}
	_, far = Neighbours(ix, "trees", 1, func(string) bool { return true }, true)
	if far != nil {
		t.Fatalf("want nil, got %v", far)
	}
}

func corpus() []Doc {
	docs := make([]Doc, 2000)
	for i := range docs {
		var b strings.Builder
		for j := 0; j < 60; j++ {
			fmt.Fprintf(&b, "w%d ", (i*7+j*13)%900)
		}
		docs[i] = Doc{fmt.Sprint("d", i), b.String()}
	}
	return docs
}

func BenchmarkBuild2000(b *testing.B) {
	docs := corpus()
	for i := 0; i < b.N; i++ {
		ix := Build(docs)
		ix.Similar(docs[0].Text, 5, nil)
	}
}
