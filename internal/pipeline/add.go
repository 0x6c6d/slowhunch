package pipeline

import (
	"context"
	"fmt"
	"io"
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
	Author    string // "agent:<provider>/<model>" for written atoms and added_by
}

// Options are the per-run inputs of an add run.
type Options struct {
	Arg   string
	Stdin io.Reader

	Medium, URL, Title, Author, Note, Hunch string

	Force, DryRun, NoCommit bool
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

// Add runs one intake item from reading to commit.
func Add(ctx context.Context, d Deps, o Options) (*report.Report, error) {
	if d.Vault == nil {
		return nil, errs.New(errs.BadInput, "no vault", "")
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	writer := d.Author
	if writer == "" {
		writer = "human"
	}

	// 1. Intake.
	in, err := intake.Read(o.Arg, o.Stdin)
	if err != nil {
		return nil, err
	}
	medium, inferred := o.Medium, false
	if medium == "" {
		medium, inferred = intake.InferMedium(in), true
	}
	if err := intake.Check(in, medium, o.Title, o.Author); err != nil {
		return nil, err
	}
	normalized := intake.Normalize(in.Text, medium)
	textHash := intake.TextHash(normalized)

	ix, _, err := d.Vault.LoadIndex()
	if err != nil {
		return nil, err
	}
	if o.Hunch != "" {
		if e, ok := ix.Get(o.Hunch); !ok || e.Type != "hunch" {
			return nil, errs.New(errs.NotFound, "no hunch "+o.Hunch, "list hunches with `slowhunch list hunch`")
		}
	}

	// 2. Secrets never leave the machine.
	if intake.ContainsSecret(normalized) || intake.ContainsSecret(in.Text) {
		return nil, errs.New(errs.BadInput, "input contains a secret-like string; nothing was sent", "remove the secret and add again")
	}

	rep := report.New(vault.NewRunID(now()))
	rep.Medium = medium
	rep.MediumInferred = inferred

	// 3. Idempotency.
	for _, e := range ix.OfType("source") {
		if e.Doc.GetString("text_hash") == textHash {
			rep.Source = report.Link(e.ID)
			rep.Counts["existing"] = 1
			rep.Git = report.GitInfo{Skipped: "existing"}
			return rep, nil
		}
	}

	sid := vault.SourceID(medium, o.Author, o.Title, normalized)
	sid = vault.UniqueID(sid, func(c string) bool {
		if _, ok := ix.Get(c); ok {
			return true
		}
		return d.Vault.Exists(sourcePath(c)) || d.Vault.Exists(inboxPath(c))
	})

	// 4. Gate.
	stored := vault.FormatInbox(normalized, medium)
	delim, err := intake.NewDelimiter(string(stored) + "\n" + in.Text)
	if err != nil {
		return nil, err
	}
	it := Item{
		SourceID: sid, Medium: medium, Title: o.Title, Author: o.Author, URL: o.URL,
		Note: o.Note, Hunch: o.Hunch, Stored: stored, Plain: vault.InboxPlain(stored, medium),
		Delim: delim, Force: o.Force,
	}
	rep.Source = report.Link(sid)
	keep, reason, gu, gerr := Gate(ctx, d.Providers["gate"], d.Prompts["gate"], it)
	if gu != (provider.Usage{}) {
		rep.AddCost("gate", toUsage(gu))
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
		rep.Errors = append(rep.Errors, report.ItemError{Item: sid, Code: code, Reason: msg})
		rep.Counts["errors"] = len(rep.Errors)
		rep.Git = report.GitInfo{Skipped: "rejected"}
		return rep, nil
	}

	// 5. Extract.
	atoms, drops, eu, eerr := Extract(ctx, d.Providers["extract"], d.Prompts["extract"], it, d.Config.Caps)
	rep.AddCost("extract", toUsage(eu))
	if eerr != nil {
		code, msg := provErr(eerr)
		rep.Source = ""
		rep.Errors = append(rep.Errors, report.ItemError{Item: sid, Code: code, Reason: msg})
		rep.Counts["errors"] = len(rep.Errors)
		rep.Git = report.GitInfo{Skipped: "extract-failed"}
		return rep, nil
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
	aesthetic := vault.IsAesthetic(medium)
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
	taken := map[string]bool{}
	var plan []placed
	for k, a := range atoms {
		nbs := BuildNeighbours(a, aesthetic, cands, d.Config.Place)
		dec, pu, perr := Place(ctx, d.Providers["place"], d.Prompts["place"], a, it, nbs, d.Config.Place)
		if pu != (provider.Usage{}) {
			rep.AddCost("place", toUsage(pu))
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
			if taken[c] {
				return true
			}
			if _, ok := ix.Get(c); ok {
				return true
			}
			return d.Vault.Exists(notePath(c))
		})
		taken[id] = true
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
		return rep, nil
	}

	// 7. Outcomes, assembled in memory before anything is written.
	today := now().Format("2006-01-02")
	ap, err := applyOutcomes(ix, plan, it, writer, today, textHash, d.Config.Place, rep)
	if err != nil {
		return nil, err
	}

	// 8. Writes: every path is touched before the first write.
	noCommit := o.NoCommit || !d.Config.Git.Commit
	txn, err := d.Vault.Begin("add", vault.TxnOptions{
		RunID:          rep.RunID,
		DryRun:         o.DryRun,
		NoCommit:       noCommit,
		Committer:      d.Committer,
		PromptVersions: provider.Versions(d.Prompts),
		Now:            now,
	})
	if err != nil {
		return nil, err
	}
	paths := []string{inboxPath(sid), sourcePath(sid)}
	for _, id := range ap.newIDs {
		paths = append(paths, notePath(id))
	}
	for _, m := range ap.mods {
		paths = append(paths, m.path)
	}
	if err := txn.Touch(paths...); err != nil {
		return nil, err
	}
	if err := txn.WriteFile(inboxPath(sid), stored); err != nil {
		return nil, err
	}
	if _, err := txn.PutDoc(sourcePath(sid), ap.source, ""); err != nil {
		return nil, err
	}
	for _, id := range ap.newIDs {
		if _, err := txn.PutDoc(notePath(id), ap.notes[id], ""); err != nil {
			return nil, err
		}
	}
	for _, m := range ap.mods {
		if _, err := txn.PutDoc(m.path, m.doc, m.rev); err != nil {
			return nil, err
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

	// 9. Commit and save.
	hash, err := txn.Commit(ids, fmt.Sprintf("atoms: %d", len(ap.newIDs)))
	if err != nil {
		return nil, err
	}
	switch {
	case o.DryRun:
		rep.Git = report.GitInfo{Skipped: "dry-run"}
	case noCommit:
		rep.Git = report.GitInfo{Skipped: "no-commit"}
	case d.Committer == nil:
		rep.Git = report.GitInfo{Skipped: "no-git"}
	default:
		rep.Git = report.GitInfo{Commit: hash}
	}
	if !o.DryRun {
		if err := rep.Save(d.Vault); err != nil {
			return nil, err
		}
	}
	for _, id := range rep.IDs() {
		if o.DryRun && planned[id] {
			continue
		}
		found := false
		for _, f := range docFolders {
			if d.Vault.Exists(f + "/" + id + ".md") {
				found = true
				break
			}
		}
		if !found {
			return nil, errs.New(errs.NotFound, "report names "+id+" but it is not in the vault", "")
		}
	}
	return rep, nil
}
