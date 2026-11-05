package report

import (
	"math"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Stats is the aggregate view over the vault and its saved reports.
type Stats struct {
	AtomsByMedium     map[string]int   `json:"atoms_by_medium"`
	AtomsByStance     map[string]int   `json:"atoms_by_stance"`
	AssertsReadsRatio *float64         `json:"asserts_reads_ratio"`
	HunchesByState    map[string]int   `json:"hunches_by_state"`
	Contradictions    int              `json:"contradictions"`
	Echoes            int              `json:"echoes"`
	TokensByRole      map[string]Usage `json:"tokens_by_role"`
	Atoms             int              `json:"atoms"`
	Hunches           int              `json:"hunches"`
}

// Compute aggregates the index and reports into Stats; all maps are non-nil.
func Compute(ix *vault.Index, reports []*Report) Stats {
	s := Stats{
		AtomsByMedium:  map[string]int{},
		AtomsByStance:  map[string]int{},
		HunchesByState: map[string]int{},
		TokensByRole:   map[string]Usage{},
	}
	for _, e := range ix.OfType("note") {
		s.Atoms++
		m := ix.MediumOf(e.ID)
		if m == "" {
			m = "unknown"
		}
		s.AtomsByMedium[m]++
		s.AtomsByStance[e.Doc.GetString("stance")]++
		if e.Doc.GetString("status") == "contradiction" {
			s.Contradictions++
		}
	}
	for _, e := range ix.OfType("hunch") {
		s.Hunches++
		s.HunchesByState[e.Doc.GetString("state")]++
	}
	if reads := s.AtomsByStance["reads"]; reads > 0 {
		r := math.Round(float64(s.AtomsByStance["asserts"])/float64(reads)*10000) / 10000
		s.AssertsReadsRatio = &r
	}
	for _, r := range reports {
		s.Echoes += len(r.Echoes)
		for role, u := range r.Cost {
			c := s.TokensByRole[role]
			c.TokensIn += u.TokensIn
			c.TokensOut += u.TokensOut
			s.TokensByRole[role] = c
		}
	}
	return s
}
