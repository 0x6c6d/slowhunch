package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/search"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("search", "search notes by similarity", cmdSearch)
}

type searchResult struct {
	Query  string
	Hits   []map[string]any
	Tokens int
}

func (r searchResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"query": r.Query, "hits": r.Hits, "count": len(r.Hits), "tokens": r.Tokens})
}

func (r searchResult) Text() string {
	var b strings.Builder
	for _, h := range r.Hits {
		fmt.Fprintf(&b, "%s  %.4f  %s\n", h["id"], h["score"], h["claim"])
		for _, l := range strings.Split(fmt.Sprint(h["quote"]), "\n") {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

func cmdSearch(env *Env, args []string) (any, error) {
	fs := newFlagSet("search")
	medium := fs.String("medium", "", "source medium")
	stance := fs.String("stance", "", "note stance")
	tag := fs.String("tag", "", "tag")
	hunch := fs.String("hunch", "", "restrict to notes near this hunch")
	budget := fs.Int("budget", 0, "token budget (0 = unlimited)")
	limit := fs.Int("limit", 10, "max hits")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "search takes exactly one query", `usage: slowhunch search "<query>" [-medium M] [-stance S] [-tag X] [-hunch ID] [-budget N] [-limit N]`)
	}
	if *medium != "" && !slices.Contains(vault.Mediums, *medium) {
		return nil, errs.New(errs.BadInput, fmt.Sprintf("unknown medium %q", *medium), "one of: "+strings.Join(vault.Mediums, " "))
	}
	if *stance != "" && !slices.Contains(vault.Stances, *stance) {
		return nil, errs.New(errs.BadInput, fmt.Sprintf("unknown stance %q", *stance), "one of: "+strings.Join(vault.Stances, " "))
	}
	if *limit < 0 || *budget < 0 {
		return nil, errs.New(errs.BadInput, "limit and budget must not be negative", "")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	hops := map[string]int{}
	if *hunch != "" {
		if e, ok := ix.Get(*hunch); !ok || e.Type != "hunch" {
			return nil, errs.New(errs.NotFound, "no hunch with id "+*hunch, "try slowhunch list -type hunch")
		}
		n1, _ := ix.Neighbourhood(*hunch, 1)
		n2, _ := ix.Neighbourhood(*hunch, 2)
		for _, id := range n2 {
			hops[id] = 1
		}
		for _, id := range n1 {
			hops[id] = 0
		}
	}
	var docs []search.Doc
	for _, e := range ix.OfType("note") {
		d := e.Doc
		docs = append(docs, search.Doc{ID: e.ID, Text: d.GetString("claim") + " " + d.GetString("evidence") + " " + strings.Join(d.GetList("tags"), " ")})
	}
	sx := search.Build(docs)
	q := sx.Query(pos[0])
	now := env.Now()
	type cand struct {
		r search.Ranked
		e *vault.Entry
	}
	var cands []cand
	for _, e := range ix.OfType("note") {
		d := e.Doc
		if *medium != "" && ix.MediumOf(e.ID) != *medium {
			continue
		}
		if *stance != "" && d.GetString("stance") != *stance {
			continue
		}
		if *tag != "" && !slices.Contains(d.GetList("tags"), *tag) {
			continue
		}
		h := 0
		if *hunch != "" {
			var ok bool
			if h, ok = hops[e.ID]; !ok {
				continue
			}
		}
		c := search.Cosine(q, sx.DocVector(e.ID))
		if c <= 0 {
			continue
		}
		days := 0
		if t, err := time.Parse("2006-01-02", d.GetString("created")); err == nil {
			days = int(now.Sub(t).Hours() / 24)
		}
		cands = append(cands, cand{search.Ranked{ID: e.ID, Score: search.Score(c, days, h), Quote: d.GetString("evidence"), Locator: d.GetString("locator"), Source: d.GetString("source")}, e})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].r.Score != cands[j].r.Score {
			return cands[i].r.Score > cands[j].r.Score
		}
		return cands[i].r.ID < cands[j].r.ID
	})
	if *limit > 0 && len(cands) > *limit {
		cands = cands[:*limit]
	}
	ranked := make([]search.Ranked, len(cands))
	for i, c := range cands {
		ranked[i] = c.r
	}
	kept, tokens := search.Budget(ranked, *budget)
	hits := []map[string]any{}
	for i, r := range kept {
		d := cands[i].e.Doc
		h := map[string]any{
			"id": r.ID, "score": search.Round4(r.Score), "claim": d.GetString("claim"),
			"quote": r.Quote, "locator": r.Locator, "source": r.Source,
			"medium": ix.MediumOf(r.ID), "stance": d.GetString("stance"), "source_title": "",
		}
		if s, ok := ix.Get(r.Source); ok {
			h["source_title"] = s.Doc.GetString("title")
			if t := s.Doc.GetString("translates"); t != "" {
				if te, ok := ix.Get(t); ok {
					h["counterpart"] = map[string]any{"id": te.ID, "title": te.Doc.GetString("title")}
				}
			}
		}
		if d.GetString("stance") == "reads" {
			for _, id := range d.GetList("rereads") {
				if re, ok := ix.Get(id); ok {
					h["rereads"] = map[string]any{"id": re.ID, "quote": re.Doc.GetString("evidence"), "locator": re.Doc.GetString("locator")}
					break
				}
			}
		}
		hits = append(hits, h)
	}
	return searchResult{pos[0], hits, tokens}, nil
}
