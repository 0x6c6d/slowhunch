package search

import (
	"math"
	"strings"
	"testing"
)

func TestSearchScore(t *testing.T) {
	if g := Score(0.5, 0, 0); math.Abs(g-0.55) > 1e-9 {
		t.Fatal(g)
	}
	if g := Score(0.5, 365, 1); math.Abs(g-0.5*1.05/2) > 1e-9 {
		t.Fatal(g)
	}
	if Score(0.5, 10, 2) >= Score(0.5, 10, 1) || Score(0.5, 400, 0) >= Score(0.5, 1, 0) {
		t.Fatal("monotonicity")
	}
}

func TestSearchBudget(t *testing.T) {
	h := []Ranked{
		{ID: "a", Quote: strings.Repeat("x", 38), Locator: "ab", Source: "cd"}, // 42 bytes -> 11
		{ID: "b", Quote: strings.Repeat("x", 40)},                              // 10
		{ID: "c", Quote: "x"}, // 1
	}
	if got, n := Budget(h, 0); len(got) != 3 || n != 22 {
		t.Fatal(len(got), n)
	}
	if got, n := Budget(h, 20); len(got) != 1 || n != 11 {
		t.Fatal(len(got), n)
	}
	if got, n := Budget(h, 22); len(got) != 3 || n != 22 {
		t.Fatal(len(got), n)
	}
	if got, _ := Budget(h, 5); len(got) != 0 {
		t.Fatal("first too big")
	}
}
