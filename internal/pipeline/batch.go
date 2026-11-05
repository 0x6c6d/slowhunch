package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/report"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// MaxBatchEntries is the most entries one batch file may hold.
const MaxBatchEntries = 50

// maxBatchBytes caps the bytes read from a batch file.
const maxBatchBytes = 1 << 20

// BatchEntry is one entry line of a batch file.
type BatchEntry struct {
	Line               int    // 1-based line number in the batch file
	File               string // the file field as written
	Path               string // File resolved against the batch file's directory
	URL, Title, Author string
	Err                error // a malformed line fails this entry alone
}

// Label is the item name of the entry in a run's errors.
func (e BatchEntry) Label() string { return "line " + strconv.Itoa(e.Line) + ": " + e.File }

// ParseBatch reads a batch file: one `<file>` or `<file>|<url>|<title>|<author>`
// per line; blank lines and lines starting with # are skipped. More than
// MaxBatchEntries entries fail with TOO_LONG, none with BAD_INPUT.
func ParseBatch(path string) ([]BatchEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errs.New(errs.NotFound, "no batch file "+path, "")
		}
		return nil, errs.New(errs.BadInput, "cannot read batch file: "+err.Error(), "")
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.IsDir() {
		return nil, errs.New(errs.BadInput, path+" is a directory", "pass a batch file")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxBatchBytes+1))
	if err != nil {
		return nil, errs.New(errs.BadInput, "cannot read batch file: "+err.Error(), "")
	}
	if len(b) > maxBatchBytes {
		return nil, errs.New(errs.TooLong, "batch file is too large", fmt.Sprintf("at most %d entries per batch", MaxBatchEntries))
	}
	if !utf8.Valid(b) {
		return nil, errs.New(errs.BadInput, "batch file is not valid UTF-8", "")
	}
	dir := filepath.Dir(path)
	var out []BatchEntry
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		e := BatchEntry{Line: i + 1}
		parts := strings.Split(t, "|")
		for k := range parts {
			parts[k] = strings.TrimSpace(parts[k])
		}
		e.File = parts[0]
		switch {
		case len(parts) != 1 && len(parts) != 4:
			e.Err = errs.New(errs.BadInput, fmt.Sprintf("line %d has %d '|' separators, want 0 or 3", e.Line, len(parts)-1),
				"write <file> or <file>|<url>|<title>|<author>")
		case e.File == "":
			e.Err = errs.New(errs.BadInput, fmt.Sprintf("line %d names no file", e.Line), "write <file> or <file>|<url>|<title>|<author>")
		}
		if len(parts) == 4 {
			e.URL, e.Title, e.Author = parts[1], parts[2], parts[3]
		}
		if e.Err == nil {
			e.Path = e.File
			if !filepath.IsAbs(e.Path) {
				e.Path = filepath.Join(dir, e.Path)
			}
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, errs.New(errs.BadInput, "batch file "+path+" has no entries", "list one file per line")
	}
	if len(out) > MaxBatchEntries {
		return nil, errs.New(errs.TooLong, fmt.Sprintf("batch has %d entries, at most %d are allowed", len(out), MaxBatchEntries), "split the batch file")
	}
	return out, nil
}

// BatchResult is the outcome of a batch add, printed on stdout.
type BatchResult struct {
	RunID  string             `json:"run_id"`
	Items  []*report.Report   `json:"items"`
	Counts map[string]int     `json:"counts"`
	Errors []report.ItemError `json:"errors"`
	Git    report.GitInfo     `json:"git"`
}

// BatchState is the state of one batch run between its stages.
type BatchState struct {
	RunID   string
	Entries []BatchEntry
	// Rejected are the entries the gate rejected (keep=false), in entry order.
	Rejected []BatchEntry

	runs    []*itemRun // per entry; nil when stage 1 failed
	failed  []error    // per entry; the entry's failure, if any
	written []bool
}

// fail records the failure of entry i.
func (s *BatchState) fail(i int, err error) {
	s.failed[i] = err
}

// AddBatch runs every entry of the batch file at from in one run with one
// transaction and one commit. o.Arg, o.Stdin, o.URL, o.Title and o.Author are
// ignored; the other options apply to every entry. When every entry failed,
// the error of the first one is returned with the result in its Extra.
func AddBatch(ctx context.Context, d Deps, from string, o Options) (*BatchResult, error) {
	if d.Vault == nil {
		return nil, errs.New(errs.BadInput, "no vault", "")
	}
	entries, err := ParseBatch(from)
	if err != nil {
		return nil, err
	}
	now := d.clock()
	st := &BatchState{
		RunID: vault.NewRunID(now()), Entries: entries,
		runs: make([]*itemRun, len(entries)), failed: make([]error, len(entries)), written: make([]bool, len(entries)),
	}
	d.meter = &tokenMeter{}
	runID := func() string { return st.RunID }
	ix, _, err := d.Vault.LoadIndex()
	if err != nil {
		return nil, err
	}
	loadIx := func() (*vault.Index, error) { return ix, nil }

	// Stage 1: intake, checks, secret check and gate, per entry in order.
	sidTaken := map[string]bool{}
	var pool []*itemRun
	var poolEntry []int
	nKept := 0
	for i, e := range entries {
		if e.Err != nil {
			st.fail(i, e.Err)
			continue
		}
		if _, err := os.Stat(e.Path); err != nil {
			st.fail(i, errs.New(errs.NotFound, "no file "+e.File, "paths are relative to the batch file"))
			continue
		}
		eo := o
		eo.Arg, eo.Stdin, eo.URL, eo.Title, eo.Author = e.Path, nil, e.URL, e.Title, e.Author
		r, _, err := prepareItem(d, eo, loadIx, runID, sidTaken)
		if err != nil {
			st.fail(i, err)
			continue
		}
		st.runs[i] = r
	}

	// Caps: before any provider call, once every entry's medium is known.
	if err := checkCaps(d, st.runs); err != nil {
		return nil, err
	}
	var txn *vault.Txn
	noCommit := o.NoCommit || !d.Config.Git.Commit
	var ids []string
	planned := map[string]bool{}
	noteTaken := map[string]bool{}
	writtenHash := map[string]string{}
	nNew := 0
	dirty := false

	// process is stage 2 for entry i: extract, place and write.
	process := func(i int, r *itemRun) error {
		if r == nil || r.done {
			return nil
		}
		if sid, ok := writtenHash[r.textHash]; ok {
			r.rep.Errors = []report.ItemError{}
			r.rep.Counts = map[string]int{}
			markExisting(r.rep, sid)
			r.done = true
			return nil
		}
		var err error
		if dirty && !o.DryRun {
			if ix, _, err = d.Vault.LoadIndex(); err != nil {
				return err
			}
		}
		ap, err := extractPlace(ctx, d, r, ix, noteTaken)
		if err != nil {
			st.fail(i, err)
			return nil
		}
		if ap == nil {
			return nil
		}
		if txn == nil {
			if txn, err = d.begin(st.RunID, o.DryRun, noCommit); err != nil {
				return err
			}
		}
		if err := txn.Touch(touchPaths(r, ap)...); err != nil {
			st.fail(i, err)
			return nil
		}
		// A failure after the first write leaves pending paths for crash-resume.
		eids, ep, err := writeItem(txn, r, ap)
		if err != nil {
			return err
		}
		dirty = true
		st.written[i] = true
		writtenHash[r.textHash] = r.it.SourceID
		ids = append(ids, eids...)
		for id := range ep {
			planned[id] = true
		}
		nNew += len(ap.newIDs)
		return nil
	}

	// Gate each entry and, when it passes, extract/place/write it at once, so a
	// tokens_per_run limit is reached entry by entry. Rejected entries are pooled.
	for i, r := range st.runs {
		if r == nil || r.done {
			continue
		}
		gateItem(ctx, d, r)
		if r.rejected {
			st.Rejected = append(st.Rejected, entries[i])
			pool = append(pool, r)
			poolEntry = append(poolEntry, i)
			continue
		}
		if err := process(i, r); err != nil {
			return nil, err
		}
	}

	// Wildcard lane: the gate-rejected entries of the batch are one pool; the
	// kept ones go on to stage 2.
	for k, kept := range chooseWildcards(d.Config.Place.WildcardShare, pool) {
		recordWildcard(pool[k], entries[poolEntry[k]].Label(), kept)
		if kept {
			nKept++
		}
	}
	for k, r := range pool {
		if err := process(poolEntry[k], r); err != nil {
			return nil, err
		}
	}

	if err := countRuns(d, txn, o.DryRun, st.runs); err != nil {
		return nil, err
	}

	// One commit for the whole batch.
	res := &BatchResult{RunID: st.RunID, Items: []*report.Report{}, Counts: map[string]int{}, Errors: []report.ItemError{}}
	if txn != nil {
		hash, err := txn.Commit(ids, fmt.Sprintf("atoms: %d", nNew))
		if err != nil {
			return nil, err
		}
		res.Git = gitInfo(d, o.DryRun, noCommit, hash)
	} else {
		res.Git = report.GitInfo{Skipped: "nothing-written"}
	}
	res.Git.Pulled = o.Pulled

	var firstErr error
	nWritten, nFailed := 0, 0
	for i, e := range entries {
		rep, ferr := st.entryReport(i, res.Git)
		rep.Git.Pulled = o.Pulled
		if ferr != nil {
			code, msg := provErr(ferr)
			res.Errors = append(res.Errors, report.ItemError{Item: e.Label(), Code: code, Reason: msg})
			nFailed++
			if firstErr == nil {
				firstErr = ferr
			}
		}
		if st.written[i] {
			nWritten++
		}
		for k, n := range rep.Counts {
			res.Counts[k] += n
		}
		res.Items = append(res.Items, rep)
		if !o.DryRun {
			if err := saveEntryReport(d.Vault, rep, i+1); err != nil {
				return nil, err
			}
		}
		if st.written[i] {
			if err := checkReportIDs(d.Vault, rep, o.DryRun, planned); err != nil {
				return nil, err
			}
		}
	}
	res.Errors = append(res.Errors, o.Warnings...)
	res.Counts["entries"] = len(entries)
	res.Counts["written"] = nWritten
	res.Counts["failed"] = nFailed
	res.Counts["wildcards"] = len(pool)
	res.Counts["wildcards_kept"] = nKept
	if nWritten == 0 && nFailed == len(entries) && firstErr != nil {
		return res, batchError(firstErr, res)
	}
	return res, nil
}

// entryReport returns the v1-shaped report of entry i and its failure, if any.
func (s *BatchState) entryReport(i int, git report.GitInfo) (*report.Report, error) {
	r := s.runs[i]
	ferr := s.failed[i]
	if r == nil || (ferr != nil && !s.written[i]) {
		rep := report.New(s.RunID)
		if r != nil {
			rep = r.rep
			rep.Source = ""
		}
		code, msg := provErr(ferr)
		rep.Errors = append(rep.Errors, report.ItemError{Item: s.Entries[i].Label(), Code: code, Reason: msg})
		rep.Counts["errors"] = len(rep.Errors)
		rep.Git = report.GitInfo{Skipped: "failed"}
		return rep, ferr
	}
	rep := r.rep
	if s.written[i] {
		rep.Git = git
		return rep, nil
	}
	if rep.Counts["existing"] == 1 {
		return rep, nil
	}
	// Rejected, extract failure or no atoms: the entry failed.
	code, msg := errs.BadInput, "no atoms were kept"
	if rep.Git.Skipped == "rejected" {
		code, msg = "REJECTED", "the gate did not keep this input"
	}
	if n := len(rep.Errors); n > 0 {
		code, msg = rep.Errors[n-1].Code, rep.Errors[n-1].Reason
	}
	return rep, &errs.Error{Code: code, Message: msg}
}

// touchPaths are the paths writeItem writes for r.
func touchPaths(r *itemRun, ap *applied) []string {
	sid := r.it.SourceID
	paths := []string{inboxPath(sid), sourcePath(sid)}
	for _, id := range ap.newIDs {
		paths = append(paths, notePath(id))
	}
	for _, m := range ap.mods {
		paths = append(paths, m.path)
	}
	return paths
}

// saveEntryReport writes rep to .slowhunch/reports/<run-id>-<n>.json.
func saveEntryReport(v *vault.Vault, rep *report.Report, n int) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return v.WriteFile(fmt.Sprintf(".slowhunch/reports/%s-%d.json", rep.RunID, n), append(b, '\n'))
}

// batchError is the error of an all-failed batch: the first entry's code and
// exit code, with the batch result as extra members.
func batchError(first error, res *BatchResult) error {
	code, msg := provErr(first)
	e := &errs.Error{Code: code, Message: msg, Exit: errs.ExitCode(first)}
	if fe, ok := first.(*errs.Error); ok {
		e.Hint = fe.Hint
	}
	e.Message = "every batch entry failed; first: " + msg
	e.Extra = map[string]any{"run_id": res.RunID, "items": res.Items, "counts": res.Counts, "errors": res.Errors, "git": res.Git}
	return e
}
