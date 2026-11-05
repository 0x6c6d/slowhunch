// Package report defines the per-run report written to
// .slowhunch/reports/<run-id>.json.
//
// Struct field declaration order fixes the JSON key order. Conventions: every
// id inside a report is written as Link(id) (e.g. "[[atom-id]]") so IDs() can
// check that each exists on disk; Collisions lists the proposals of a collide run; Wildcards lists the gate-rejected items of the run.
package report

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

type Usage struct {
	TokensIn  int `json:"tokens_in"`
	TokensOut int `json:"tokens_out"`
}

type AtomRef struct {
	ID      string `json:"id"`
	Claim   string `json:"claim"`
	Stance  string `json:"stance"`
	Locator string `json:"locator"`
	Action  string `json:"action"`
	Target  string `json:"target,omitempty"`
}

type Contradiction struct {
	Atom        string   `json:"atom"`
	Contradicts string   `json:"contradicts"`
	Hunches     []string `json:"hunches"`
	Cosine      float64  `json:"cosine"`
}

type HunchMove struct {
	Hunch    string `json:"hunch"`
	Note     string `json:"note"`
	Relation string `json:"relation"`
}

type Corroboration struct {
	Atom   string `json:"atom"`
	Source string `json:"source"`
	Date   string `json:"date"`
}

type Echo struct {
	Atom   string  `json:"atom"`
	Echoes string  `json:"echoes"`
	Cosine float64 `json:"cosine"`
}

// Wildcard is one gate-rejected item: Item is the source id (single add) or
// "line <n>: <file>" (batch entry), Reason the gate's reason, Kept whether the
// wildcard lane wrote it anyway.
type Wildcard struct {
	Item   string `json:"item"`
	Reason string `json:"reason"`
	Kept   bool   `json:"kept"`
}

// Collision is one kept collide pair: Items are the two linked ids, Proposal
// the linked proposal written for them.
type Collision struct {
	Items    []string `json:"items"`
	Proposal string   `json:"proposal"`
}

type ItemError struct {
	Item   string `json:"item"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type GitInfo struct {
	Commit  string `json:"commit"`
	Skipped string `json:"skipped,omitempty"`
}

type Report struct {
	RunID          string           `json:"run_id"`
	Source         string           `json:"source"`
	Medium         string           `json:"medium"`
	MediumInferred bool             `json:"medium_inferred"`
	Contradictions []Contradiction  `json:"contradictions"`
	HunchesMoved   []HunchMove      `json:"hunches_moved"`
	Atoms          []AtomRef        `json:"atoms"`
	Corroborations []Corroboration  `json:"corroborations"`
	Echoes         []Echo           `json:"echoes"`
	Wildcards      []Wildcard       `json:"wildcards"`
	Collisions     []Collision      `json:"collisions"`
	Counts         map[string]int   `json:"counts"`
	Errors         []ItemError      `json:"errors"`
	Cost           map[string]Usage `json:"cost"`
	Git            GitInfo          `json:"git"`
}

// New returns a report whose slices and maps are all non-nil.
func New(runID string) *Report {
	return &Report{
		RunID:          runID,
		Contradictions: []Contradiction{},
		HunchesMoved:   []HunchMove{},
		Atoms:          []AtomRef{},
		Corroborations: []Corroboration{},
		Echoes:         []Echo{},
		Wildcards:      []Wildcard{},
		Collisions:     []Collision{},
		Counts:         map[string]int{},
		Errors:         []ItemError{},
		Cost:           map[string]Usage{},
	}
}

// Link returns the wikilink form of id.
func Link(id string) string { return "[[" + id + "]]" }

var linkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// IDs returns every id inside [[...]] in the marshalled report, deduplicated and sorted.
func (r *Report) IDs() []string {
	b, _ := json.Marshal(r)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range linkRe.FindAllStringSubmatch(string(b), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// AddCost sums usage per role.
func (r *Report) AddCost(role string, u Usage) {
	if r.Cost == nil {
		r.Cost = map[string]Usage{}
	}
	c := r.Cost[role]
	c.TokensIn += u.TokensIn
	c.TokensOut += u.TokensOut
	r.Cost[role] = c
}

const dir = ".slowhunch/reports"

// Save writes the report to .slowhunch/reports/<run-id>.json.
func (r *Report) Save(v *vault.Vault) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return v.WriteFile(dir+"/"+r.RunID+".json", append(b, '\n'))
}

// LoadAll loads every report, sorted by file name; a missing dir yields none.
func LoadAll(v *vault.Vault) ([]*Report, error) {
	p, err := v.Abs(dir)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(p)
	if errors.Is(err, os.ErrNotExist) {
		return []*Report{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []*Report{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := v.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return nil, err
		}
		r := New("")
		if err := json.Unmarshal(b, r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
