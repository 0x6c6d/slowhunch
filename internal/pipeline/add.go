package pipeline

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Deps are the collaborators of an add run.
type Deps struct {
	Vault     *vault.Vault
	Config    vault.Config
	Providers map[string]provider.Provider // keys gate, extract, place
	Prompts   map[string]provider.Prompt
	Committer vault.Committer // nil = no git
	Now       func() time.Time
	// ProviderFor, when set, returns the provider for a role and the item's medium
	// and takes precedence over Providers.
	ProviderFor func(role, medium string) (provider.Provider, error)
	// AuthorFor, when set, returns the writer string for the extract provider used.
	AuthorFor func(role, medium string) string
	Author    string // "agent:<provider>/<model>" for written atoms and added_by

	meter *tokenMeter // run-wide token meter; set by Add and AddBatch
}

// Options are the per-run inputs of an add run.
type Options struct {
	Arg   string
	Stdin io.Reader

	Medium, URL, Title, Author, Note, Hunch string

	Force, DryRun, NoCommit bool

	// PDFCommand is pdf.command from config; empty leaves .pdf files UNSUPPORTED.
	PDFCommand []string
}

var docFolders = []string{"notes", "sources", "hunches", "proposals", "topics"}

func toUsage(u provider.Usage) report.Usage {
	return report.Usage{TokensIn: u.TokensIn, TokensOut: u.TokensOut}
}

func provErr(err error) (string, string) {
	if e, ok := err.(*errs.Error); ok {
		return e.Code, e.Message
	}
	return errs.ProviderError, err.Error()
}

// foreignConflict fails with CONFLICT when any of paths has uncommitted
// changes that are not pending from an earlier (crashed) run. It guards the
// natural inbox/source paths of a new source: a foreign file there must not be
// silently side-stepped by a collision suffix. Without commits it is a no-op,
// matching Txn.Touch.
func foreignConflict(d Deps, noCommit bool, paths ...string) error {
	if noCommit || d.Committer == nil {
		return nil
	}
	st, err := d.Vault.LoadState()
	if err != nil {
		return err
	}
	pending := map[string]bool{}
	for _, p := range st.Pending {
		pending[p] = true
	}
	dirty, err := d.Committer.Dirty(paths)
	if err != nil {
		return err
	}
	var bad []string
	for _, p := range dirty {
		if !pending[p] {
			bad = append(bad, p)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		e := errs.New(errs.Conflict, "uncommitted changes in "+strings.Join(bad, ", "), "commit or revert them first")
		e.Extra = map[string]any{"paths": bad}
		return e
	}
	return nil
}

func (d Deps) provider(role, medium string) (provider.Provider, error) {
	if d.ProviderFor != nil {
		return d.ProviderFor(role, medium)
	}
	return d.Providers[role], nil
}

func (d Deps) clock() func() time.Time {
	if d.Now == nil {
		return time.Now
	}
	return d.Now
}

// itemRun is one intake item moving between the stages of an add run.
type itemRun struct {
	rep                     *report.Report
	normalized, textHash    string
	stored                  []byte
	it                      Item
	gateP, extractP, placeP provider.Provider
	writer                  string
	noCommit                bool
	// done means rep is final: the item is existing, rejected or produced no atoms.
	done bool
	// rejected means the gate answered keep=false (not a gate error).
	rejected bool
	// called means a provider was called for this item.
	called bool
	// wildcard means a rejected item kept by the wildcard lane; its atoms get the tag wildcard.
	wildcard bool
	// capHit means a provider call of this item was refused by tokens_per_run.
	capHit bool
}

// prepareItem is stage 1 up to the gate: intake, checks, secret check,
// idempotency and source id. newRunID is called once the report is created;
// loadIndex supplies the index after the input checks; sidTaken, when non-nil, holds source ids already claimed by this run.
func prepareItem(d Deps, o Options, loadIndex func() (*vault.Index, error), newRunID func() string, sidTaken map[string]bool) (*itemRun, *vault.Index, error) {
	writer := d.Author
	if writer == "" {
		writer = "human"
	}

	// 1. Intake.
	in, err := intake.ReadWith(o.Arg, o.Stdin, intake.ReadOptions{PDFCommand: o.PDFCommand})
	if err != nil {
		return nil, nil, err
	}
	medium, inferred := o.Medium, false
	if medium == "" {
		medium, inferred = intake.InferMedium(in), true
	}
	if err := intake.Check(in, medium, o.Title, o.Author); err != nil {
		return nil, nil, err
	}
	normalized := intake.Normalize(in.Text, medium)
	textHash := intake.TextHash(normalized)

	ix, err := loadIndex()
	if err != nil {
		return nil, nil, err
	}
	if o.Hunch != "" {
		if e, ok := ix.Get(o.Hunch); !ok || e.Type != "hunch" {
			return nil, nil, errs.New(errs.NotFound, "no hunch "+o.Hunch, "list hunches with `slowhunch list hunch`")
		}
	}

	// 2. Secrets never leave the machine.
	if intake.ContainsSecret(normalized) || intake.ContainsSecret(in.Text) {
		return nil, nil, errs.New(errs.BadInput, "input contains a secret-like string; nothing was sent", "remove the secret and add again")
	}

	rep := report.New(newRunID())
	rep.Medium = medium
	rep.MediumInferred = inferred
	r := &itemRun{rep: rep, normalized: normalized, textHash: textHash, writer: writer}

	// 3. Idempotency.
	for _, e := range ix.OfType("source") {
		if e.Doc.GetString("text_hash") == textHash {
			markExisting(rep, e.ID)
			r.done = true
			return r, ix, nil
		}
	}

	sid := vault.SourceID(medium, o.Author, o.Title, normalized)
	r.noCommit = o.NoCommit || !d.Config.Git.Commit
	if err := foreignConflict(d, r.noCommit, inboxPath(sid), sourcePath(sid)); err != nil {
		return nil, nil, err
	}
	sid = vault.UniqueID(sid, func(c string) bool {
		if sidTaken[c] {
			return true
		}
		if _, ok := ix.Get(c); ok {
			return true
		}
		return d.Vault.Exists(sourcePath(c)) || d.Vault.Exists(inboxPath(c))
	})
	if sidTaken != nil {
		sidTaken[sid] = true
	}

	r.stored = vault.FormatInbox(normalized, medium)
	delim, err := intake.NewDelimiter(string(r.stored) + "\n" + in.Text)
	if err != nil {
		return nil, nil, err
	}
	r.it = Item{
		SourceID: sid, Medium: medium, Title: o.Title, Author: o.Author, URL: o.URL,
		Note: o.Note, Hunch: o.Hunch, Stored: r.stored, Plain: vault.InboxPlain(r.stored, medium),
		Delim: delim, Force: o.Force,
	}
	rep.Source = report.Link(sid)
	if r.gateP, err = d.provider("gate", medium); err != nil {
		return nil, nil, err
	}
	if r.extractP, err = d.provider("extract", medium); err != nil {
		return nil, nil, err
	}
	if r.placeP, err = d.provider("place", medium); err != nil {
		return nil, nil, err
	}
	if d.meter != nil {
		limit := d.Config.Caps.ByMedium[medium].TokensPerRun
		r.gateP = &meteredProvider{inner: r.gateP, meter: d.meter, limit: limit, hit: &r.capHit}
		r.extractP = &meteredProvider{inner: r.extractP, meter: d.meter, limit: limit, hit: &r.capHit}
		r.placeP = &meteredProvider{inner: r.placeP, meter: d.meter, limit: limit, hit: &r.capHit}
	}
	if d.AuthorFor != nil {
		if a := d.AuthorFor("extract", medium); a != "" {
			r.writer = a
		}
	}
	return r, ix, nil
}

// markExisting finalizes rep for an input whose text is already the source id.
func markExisting(rep *report.Report, id string) {
	rep.Source = report.Link(id)
	rep.Counts["existing"] = 1
	rep.Git = report.GitInfo{Skipped: "existing"}
}

// gateItem is the gate step; a rejected item is marked done.
func gateItem(ctx context.Context, d Deps, r *itemRun) {
	rep := r.rep
	r.called = true
	keep, reason, gu, gerr := Gate(ctx, r.gateP, d.Prompts["gate"], r.it)
	if gu != (provider.Usage{}) {
		rep.AddCost("gate", toUsage(gu))
	}
	if r.capHit {
		r.capFail()
		return
	}
	if gerr != nil || !keep {
		code, msg := "REJECTED", reason
		if gerr != nil {
			code, msg = provErr(gerr)
		}
		if msg == "" {
			msg = "the gate did not keep this input"
		}
		rep.Source = ""
		rep.Counts["rejected"] = 1
		rep.Errors = append(rep.Errors, report.ItemError{Item: r.it.SourceID, Code: code, Reason: msg})
		rep.Counts["errors"] = len(rep.Errors)
		rep.Git = report.GitInfo{Skipped: "rejected"}
		r.done = true
		r.rejected = gerr == nil
	}
}

// extractPlace is stage 2 up to the writes: extract, place and the outcomes
// assembled in memory. It returns nil when the item ends without writes.
// noteTaken holds note ids already claimed by this run.
func extractPlace(ctx context.Context, d Deps, r *itemRun, ix *vault.Index, noteTaken map[string]bool) (*applied, error) {
	rep, it, sid := r.rep, r.it, r.it.SourceID

	// 5. Extract.
	atoms, drops, eu, eerr := Extract(ctx, r.extractP, d.Prompts["extract"], it, d.Config.Caps)
	rep.AddCost("extract", toUsage(eu))
	if r.capHit {
		r.capFail()
		return nil, nil
	}
	if r.wildcard {
		for k := range atoms {
			atoms[k].Tags = addTag(atoms[k].Tags, wildcardTag)
		}
	}
	if eerr != nil {
		code, msg := provErr(eerr)
		rep.Source = ""
		rep.Errors = append(rep.Errors, report.ItemError{Item: sid, Code: code, Reason: msg})
		rep.Counts["errors"] = len(rep.Errors)
		rep.Git = report.GitInfo{Skipped: "extract-failed"}
		r.done = true
		return nil, nil
	}
	dropped := map[int]bool{}
	for _, dr := range drops {
		dropped[dr.Index] = true
		rep.Errors = append(rep.Errors, report.ItemError{Item: fmt.Sprintf("atom %d", dr.Index), Code: dr.Code, Reason: dr.Reason})
	}
	nDropped := len(drops)
	// Original model indices of the kept atoms (CheckAtoms keeps order).
	var keptIdx []int
	for i := 0; len(keptIdx) < len(atoms); i++ {
		if !dropped[i] {
			keptIdx = append(keptIdx, i)
		}
	}

	// 6. Place.
	aesthetic := vault.IsAesthetic(it.Medium)
	var cands []Candidate
	for _, e := range ix.OfType("note") {
		tags := e.Doc.GetList("tags")
		cands = append(cands, Candidate{
			ID:        e.ID,
			Text:      e.Doc.GetString("claim") + "\n" + e.Doc.GetString("evidence") + "\n" + strings.Join(tags, " "),
			Evidence:  e.Doc.GetString("evidence"),
			Stance:    e.Doc.GetString("stance"),
			Tags:      tags,
			Aesthetic: vault.IsAesthetic(ix.MediumOf(e.ID)),
		})
	}
	var plan []placed
	for k, a := range atoms {
		nbs := BuildNeighbours(a, aesthetic, cands, d.Config.Place)
		dec, pu, perr := Place(ctx, r.placeP, d.Prompts["place"], a, it, nbs, d.Config.Place)
		if pu != (provider.Usage{}) {
			rep.AddCost("place", toUsage(pu))
		}
		if r.capHit {
			r.capFail()
			return nil, nil
		}
		if perr != nil {
			code, msg := provErr(perr)
			rep.Errors = append(rep.Errors, report.ItemError{Item: fmt.Sprintf("atom %d", keptIdx[k]), Code: code, Reason: msg})
			nDropped++
			continue
		}
		if dec.Action == "duplicate" {
			// No new note: the target gets a corroboration line.
			plan = append(plan, placed{Atom: a, Decision: dec})
			continue
		}
		id := vault.UniqueID(vault.AtomID(a.Stance, sid, a.Locator, a.Claim), func(c string) bool {
			if noteTaken[c] {
				return true
			}
			if _, ok := ix.Get(c); ok {
				return true
			}
			return d.Vault.Exists(notePath(c))
		})
		noteTaken[id] = true
		plan = append(plan, placed{ID: id, Atom: a, Decision: dec})
		cands = append(cands, Candidate{
			ID:        id,
			Text:      a.Claim + "\n" + a.Evidence + "\n" + strings.Join(a.Tags, " "),
			Evidence:  a.Evidence,
			Stance:    a.Stance,
			Tags:      a.Tags,
			Aesthetic: aesthetic,
		})
	}
	rep.Counts["atoms"] = len(plan)
	rep.Counts["dropped"] = nDropped
	rep.Counts["errors"] = len(rep.Errors)
	if len(plan) == 0 {
		rep.Source = ""
		rep.Git = report.GitInfo{Skipped: "no-atoms"}
		r.done = true
		return nil, nil
	}

	// 7. Outcomes, assembled in memory before anything is written.
	today := d.clock()().Format("2006-01-02")
	return applyOutcomes(ix, plan, it, r.writer, today, r.textHash, d.Config.Place, rep)
}

// writeItem writes one item inside txn; every path is touched before the
// first write. It returns the ids for the commit subject and the planned ids.
func writeItem(txn *vault.Txn, r *itemRun, ap *applied) ([]string, map[string]bool, error) {
	sid := r.it.SourceID
	if err := txn.Touch(touchPaths(r, ap)...); err != nil {
		return nil, nil, err
	}
	if err := txn.WriteFile(inboxPath(sid), r.stored); err != nil {
		return nil, nil, err
	}
	if _, err := txn.PutDoc(sourcePath(sid), ap.source, ""); err != nil {
		return nil, nil, err
	}
	for _, id := range ap.newIDs {
		if _, err := txn.PutDoc(notePath(id), ap.notes[id], ""); err != nil {
			return nil, nil, err
		}
	}
	for _, m := range ap.mods {
		if _, err := txn.PutDoc(m.path, m.doc, m.rev); err != nil {
			return nil, nil, err
		}
	}
	ids := []string{sid}
	planned := map[string]bool{sid: true}
	for _, id := range ap.newIDs {
		ids = append(ids, id)
		planned[id] = true
	}
	for _, m := range ap.mods {
		ids = append(ids, m.id)
	}
	return ids, planned, nil
}

// gitInfo is the git member of a report for a run that wrote files.
func gitInfo(d Deps, dryRun, noCommit bool, hash string) report.GitInfo {
	switch {
	case dryRun:
		return report.GitInfo{Skipped: "dry-run"}
	case noCommit:
		return report.GitInfo{Skipped: "no-commit"}
	case d.Committer == nil:
		return report.GitInfo{Skipped: "no-git"}
	default:
		return report.GitInfo{Commit: hash}
	}
}

// checkReportIDs fails when rep names an id that is not in the vault; in
// dry-run the planned ids are exempt.
func checkReportIDs(v *vault.Vault, rep *report.Report, dryRun bool, planned map[string]bool) error {
	for _, id := range rep.IDs() {
		if dryRun && planned[id] {
			continue
		}
		found := false
		for _, f := range docFolders {
			if v.Exists(f + "/" + id + ".md") {
				found = true
				break
			}
		}
		if !found {
			return errs.New(errs.NotFound, "report names "+id+" but it is not in the vault", "")
		}
	}
	return nil
}

func (d Deps) begin(runID string, dryRun, noCommit bool) (*vault.Txn, error) {
	return d.Vault.Begin("add", vault.TxnOptions{
		RunID:          runID,
		DryRun:         dryRun,
		NoCommit:       noCommit,
		Committer:      d.Committer,
		PromptVersions: provider.Versions(d.Prompts),
		Now:            d.clock(),
	})
}

// Add runs one intake item from reading to commit.
func Add(ctx context.Context, d Deps, o Options) (*report.Report, error) {
	if d.Vault == nil {
		return nil, errs.New(errs.BadInput, "no vault", "")
	}
	now := d.clock()
	d.meter = &tokenMeter{}
	r, ix, err := prepareItem(d, o, func() (*vault.Index, error) {
		ix, _, err := d.Vault.LoadIndex()
		return ix, err
	}, func() string { return vault.NewRunID(now()) }, nil)
	if err != nil {
		return nil, err
	}
	if r.done {
		return r.rep, nil
	}
	if err := checkCaps(d, []*itemRun{r}); err != nil {
		return nil, err
	}
	if limit := d.Config.Caps.ByMedium[r.it.Medium].TokensPerRun; limit > 0 && d.meter.used >= limit {
		return nil, capErr(r.it.Medium, limit)
	}
	runs := []*itemRun{r}
	// 4. Gate, then the wildcard lane for a rejected item.
	gateItem(ctx, d, r)
	if r.rejected {
		kept := chooseWildcards(d.Config.Place.WildcardShare, runs)
		recordWildcard(r, r.it.SourceID, kept[0])
	}
	if r.done {
		return r.rep, countRuns(d, nil, o.DryRun, runs)
	}
	ap, err := extractPlace(ctx, d, r, ix, map[string]bool{})
	if err != nil {
		return nil, err
	}
	if ap == nil {
		return r.rep, countRuns(d, nil, o.DryRun, runs)
	}
	rep := r.rep

	// 8. Writes.
	txn, err := d.begin(rep.RunID, o.DryRun, r.noCommit)
	if err != nil {
		return nil, err
	}
	ids, planned, err := writeItem(txn, r, ap)
	if err != nil {
		return nil, err
	}

	// 9. Commit and save.
	if err := countRuns(d, txn, o.DryRun, runs); err != nil {
		return nil, err
	}
	hash, err := txn.Commit(ids, fmt.Sprintf("atoms: %d", len(ap.newIDs)))
	if err != nil {
		return nil, err
	}
	rep.Git = gitInfo(d, o.DryRun, r.noCommit, hash)
	if !o.DryRun {
		if err := rep.Save(d.Vault); err != nil {
			return nil, err
		}
	}
	if err := checkReportIDs(d.Vault, rep, o.DryRun, planned); err != nil {
		return nil, err
	}
	return rep, nil
}

// checkCaps fails with CAP_REACHED, before any provider call, when a medium of
// the run has used up caps.by_medium.<medium>.runs_per_day today.
func checkCaps(d Deps, runs []*itemRun) error {
	seen := map[string]bool{}
	var mediums []string
	for _, r := range runs {
		if r == nil || r.done || seen[r.it.Medium] {
			continue
		}
		seen[r.it.Medium] = true
		mediums = append(mediums, r.it.Medium)
	}
	sort.Strings(mediums)
	var st vault.State
	loaded := false
	today := d.clock()().Format("2006-01-02")
	for _, m := range mediums {
		limit := d.Config.Caps.ByMedium[m].RunsPerDay
		if limit <= 0 {
			continue
		}
		if !loaded {
			var err error
			if st, err = d.Vault.LoadState(); err != nil {
				return err
			}
			loaded = true
		}
		if st.RunsByDay[today][m] >= limit {
			return errs.New(errs.CapReached, fmt.Sprintf("medium %s reached its limit of %d runs per day", m, limit),
				"raise caps.by_medium."+m+".runs_per_day or wait until tomorrow")
		}
	}
	return nil
}

// calledMediums are the distinct capped mediums of runs that called a provider
// (mediums without a limit are not counted, so uncapped vaults keep state.json untouched).
func calledMediums(d Deps, runs []*itemRun) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range runs {
		if r != nil && r.called && d.Config.Caps.ByMedium[r.it.Medium].RunsPerDay > 0 && !seen[r.it.Medium] {
			seen[r.it.Medium] = true
			out = append(out, r.it.Medium)
		}
	}
	sort.Strings(out)
	return out
}

// countRuns records the run in the state: through txn when there is one,
// else directly. A dry run or a run without provider calls counts nothing.
func countRuns(d Deps, txn *vault.Txn, dryRun bool, runs []*itemRun) error {
	mediums := calledMediums(d, runs)
	if dryRun || len(mediums) == 0 {
		return nil
	}
	today := d.clock()().Format("2006-01-02")
	if txn != nil {
		txn.AddRuns(today, mediums)
		return nil
	}
	st, err := d.Vault.LoadState()
	if err != nil {
		return err
	}
	st.AddRuns(today, mediums)
	return d.Vault.SaveState(st)
}

// tokenMeter counts the tokens in and out of every provider call of a run.
type tokenMeter struct{ used int }

func capErr(medium string, limit int) error {
	return errs.New(errs.CapReached, fmt.Sprintf("medium %s reached its limit of %d tokens per run", medium, limit),
		"raise caps.by_medium."+medium+".tokens_per_run")
}

// meteredProvider refuses a call once the run's meter has reached limit
// (limit > 0); the call in flight always finishes and is counted.
type meteredProvider struct {
	inner provider.Provider
	meter *tokenMeter
	limit int
	hit   *bool
}

func (m *meteredProvider) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	if m.limit > 0 && m.meter.used >= m.limit {
		*m.hit = true
		return provider.Response{}, capErr(req.Medium, m.limit)
	}
	resp, err := m.inner.Complete(ctx, req)
	m.meter.used += resp.TokensIn + resp.TokensOut
	return resp, err
}

// capFail ends the item without writes: a call was refused by tokens_per_run.
func (r *itemRun) capFail() {
	rep := r.rep
	rep.Source = ""
	rep.Counts["atoms"] = 0
	rep.Errors = append(rep.Errors, report.ItemError{Item: r.it.SourceID, Code: errs.CapReached,
		Reason: "the tokens_per_run limit of medium " + r.it.Medium + " was reached"})
	rep.Counts["errors"] = len(rep.Errors)
	rep.Git = report.GitInfo{Skipped: "cap-reached"}
	r.done = true
	r.rejected = false
}
