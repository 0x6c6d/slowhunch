// Package synth holds the synthesis steps: collide pair selection and review queries.
package synth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Review is the read-only result of the review queries.
type Review struct {
	StaleHunches []string `json:"stale_hunches"`
	Orphans      []string `json:"orphans"`
	Unplaced     []string `json:"unplaced"`
}

const dateLayout = "2006-01-02"

// RunReview answers the review queries over a loaded vault. It only reads.
func RunReview(v *vault.Vault, ix *vault.Index, today time.Time, staleDays int) (Review, error) {
	r := Review{StaleHunches: []string{}, Orphans: []string{}, Unplaced: []string{}}
	loc := today.Location()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	audit := auditTimes(v, loc)

	for _, e := range ix.OfType("hunch") {
		st := e.Doc.GetString("state")
		if st != "open" && st != "warming" {
			continue
		}
		var last time.Time
		bump := func(t time.Time) {
			if t.After(last) {
				last = t
			}
		}
		if t, err := time.Parse(dateLayout, e.Doc.GetString("created")); err == nil {
			bump(t)
		}
		for _, m := range e.Doc.GetMaps("evidence") {
			if t, err := time.Parse(dateLayout, m["date"]); err == nil {
				bump(t)
			}
		}
		for _, t := range audit[e.Path] {
			bump(t)
		}
		if last.IsZero() {
			continue
		}
		if todayDate.Sub(last) > time.Duration(staleDays)*24*time.Hour {
			r.StaleHunches = append(r.StaleHunches, e.ID)
		}
	}

	named := map[string]bool{}
	reports, err := report.LoadAll(v)
	if err != nil {
		return r, err
	}
	for _, rp := range reports {
		for _, ec := range rp.Echoes {
			named[strings.TrimSuffix(strings.TrimPrefix(ec.Atom, "[["), "]]")] = true
			named[strings.TrimSuffix(strings.TrimPrefix(ec.Echoes, "[["), "]]")] = true
		}
	}

	for _, e := range ix.OfType("note") {
		noBack := len(ix.Backlinks(e.ID)) == 0
		if noBack && len(ix.Outgoing(e.ID)) == 0 {
			r.Orphans = append(r.Orphans, e.ID)
		}
		if noBack && vault.IsAesthetic(ix.MediumOf(e.ID)) &&
			len(e.Doc.GetList("rereads")) == 0 && !named[e.ID] {
			r.Unplaced = append(r.Unplaced, e.ID)
		}
	}
	sort.Strings(r.StaleHunches)
	sort.Strings(r.Orphans)
	sort.Strings(r.Unplaced)
	return r, nil
}

// auditTimes maps each path in the audit log to the local dates of its lines,
// as UTC midnights. A missing log or a malformed line is skipped.
func auditTimes(v *vault.Vault, loc *time.Location) map[string][]time.Time {
	out := map[string][]time.Time{}
	b, err := v.ReadFile(".slowhunch/audit.log")
	if err != nil {
		return out
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e vault.AuditEntry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, e.Time)
		if err != nil {
			continue
		}
		t = t.In(loc)
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		for _, p := range e.Paths {
			out[p] = append(out[p], d)
		}
	}
	return out
}
