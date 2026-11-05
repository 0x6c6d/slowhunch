package vault

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Committer is the git surface a transaction needs; *gitops.Repo satisfies it.
type Committer interface {
	Dirty(paths []string) ([]string, error)
	Commit(paths []string, subject, body string) (string, error)
}

// TxnOptions configures a transaction.
type TxnOptions struct {
	RunID          string // "" -> NewRunID(Now())
	DryRun         bool
	NoCommit       bool      // --no-commit or config git.commit=false
	Committer      Committer // nil = git unavailable: behave like NoCommit
	PromptVersions map[string]string
	Now            func() time.Time // nil -> time.Now
}

// Txn groups every write of one command run into a single commit.
type Txn struct {
	v       *Vault
	command string
	o       TxnOptions
	runID   string
	state   State
	pending map[string]bool
	written map[string]bool
	done    bool
}

// NewRunID returns "YYYYMMDD-HHMMSS-" followed by 6 random hex digits.
func NewRunID(now time.Time) string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return now.Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// Begin starts a transaction; pending paths left by a crashed run are adopted.
func (v *Vault) Begin(command string, o TxnOptions) (*Txn, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.RunID == "" {
		o.RunID = NewRunID(o.Now())
	}
	st, err := v.LoadState()
	if err != nil {
		return nil, err
	}
	t := &Txn{v: v, command: command, o: o, runID: o.RunID, state: st,
		pending: map[string]bool{}, written: map[string]bool{}}
	for _, p := range st.Pending {
		if c, err := clean(p); err == nil {
			t.pending[c] = true
		}
	}
	return t, nil
}

// AddRuns counts a run per medium on day in the state saved by Commit.
func (t *Txn) AddRuns(day string, mediums []string) { t.state.AddRuns(day, mediums) }

// RunID returns the run id of this transaction.
func (t *Txn) RunID() string { return t.runID }

func (t *Txn) commitEnabled() bool { return !t.o.NoCommit && t.o.Committer != nil }

// internal reports paths that are git-ignored bookkeeping, never committed.
func internal(c string) bool {
	return c == ".slowhunch/state.json" || c == ".slowhunch/audit.log" ||
		c == ".slowhunch/reports" || strings.HasPrefix(c, ".slowhunch/reports/")
}

func (t *Txn) usable() error {
	if t.done {
		return errs.New(errs.BadInput, "transaction already committed", "")
	}
	return nil
}

// Touch declares paths this run will write and fails with CONFLICT when any of
// them has uncommitted changes not owned by this run.
func (t *Txn) Touch(paths ...string) error {
	if err := t.usable(); err != nil {
		return err
	}
	var check []string
	for _, p := range paths {
		c, err := clean(p)
		if err != nil {
			return err
		}
		if internal(c) || t.pending[c] || t.written[c] {
			continue
		}
		check = append(check, c)
	}
	if !t.commitEnabled() || len(check) == 0 {
		return nil
	}
	dirty, err := t.o.Committer.Dirty(check)
	if err != nil {
		return err
	}
	var bad []string
	for _, d := range dirty {
		if !t.pending[d] && !t.written[d] {
			bad = append(bad, d)
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

// markPending records paths in State.Pending and saves it before a real write.
func (t *Txn) markPending(paths ...string) error {
	changed := false
	for _, p := range paths {
		c, err := clean(p)
		if err != nil {
			return err
		}
		if internal(c) || t.pending[c] {
			continue
		}
		t.pending[c] = true
		changed = true
	}
	if !changed {
		return nil
	}
	t.state.Pending = sortedKeys(t.pending)
	return t.v.SaveState(t.state)
}

func (t *Txn) record(paths ...string) {
	for _, p := range paths {
		if c, err := clean(p); err == nil {
			t.written[c] = true
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// prepare validates, touches and (outside dry-run) marks paths pending.
func (t *Txn) prepare(paths ...string) error {
	for _, p := range paths {
		if _, err := t.v.Abs(p); err != nil {
			return err
		}
	}
	if err := t.Touch(paths...); err != nil {
		return err
	}
	if t.o.DryRun {
		return nil
	}
	return t.markPending(paths...)
}

// WriteFile writes rel inside the transaction.
func (t *Txn) WriteFile(rel string, data []byte) error {
	if err := t.prepare(rel); err != nil {
		return err
	}
	if t.o.DryRun {
		if isInbox(rel) && t.v.Exists(rel) {
			return errs.New(errs.Exists, "inbox files are immutable: "+rel, "")
		}
		t.record(rel)
		return nil
	}
	if err := t.v.WriteFile(rel, data); err != nil {
		return err
	}
	t.record(rel)
	return nil
}

// PutDoc writes d to rel guarded by expectRev and returns the (would-be) rev.
func (t *Txn) PutDoc(rel string, d *frontmatter.Doc, expectRev string) (string, error) {
	if err := t.prepare(rel); err != nil {
		return "", err
	}
	if !t.o.DryRun {
		rev, err := t.v.PutDoc(rel, d, expectRev)
		if err != nil {
			return "", err
		}
		t.record(rel)
		return rev, nil
	}
	if t.v.Exists(rel) {
		cur, err := t.v.ReadFile(rel)
		if err != nil {
			return "", err
		}
		stored, err := frontmatter.Parse(cur)
		if err != nil {
			return "", err
		}
		if expectRev == "" {
			return "", errs.New(errs.Exists, "file exists: "+rel, "pass the current rev to update it")
		}
		curRev := frontmatter.Rev(stored)
		if expectRev != curRev {
			e := errs.New(errs.ConflictRev, "stale rev for "+rel+": expected "+expectRev+", current "+curRev,
				"re-read the file and retry with the current rev")
			e.Extra = map[string]any{"current": string(cur), "rev": curRev}
			return "", e
		}
		if stored.GetString("type") == "note" || d.GetString("type") == "note" {
			if stored.GetString("evidence") != d.GetString("evidence") {
				return "", errs.New(errs.BadInput, "evidence is never rewritten: "+rel, "write a new note instead")
			}
		}
	}
	rev := frontmatter.Rev(d)
	d.Set("rev", rev)
	t.record(rel)
	return rev, nil
}

// Move renames from to to inside the transaction.
func (t *Txn) Move(from, to string) error {
	if err := t.prepare(from, to); err != nil {
		return err
	}
	if t.o.DryRun {
		if !t.v.Exists(from) {
			return errs.New(errs.NotFound, "not found: "+from, "")
		}
		if t.v.Exists(to) {
			return errs.New(errs.Exists, "target exists: "+to, "")
		}
		t.record(from, to)
		return nil
	}
	if err := t.v.Move(from, to); err != nil {
		return err
	}
	t.record(from, to)
	return nil
}

// Remove deletes rel inside the transaction.
func (t *Txn) Remove(rel string) error {
	if err := t.prepare(rel); err != nil {
		return err
	}
	if t.o.DryRun {
		if !t.v.Exists(rel) {
			return errs.New(errs.NotFound, "not found: "+rel, "")
		}
		t.record(rel)
		return nil
	}
	if err := t.v.Remove(rel); err != nil {
		return err
	}
	t.record(rel)
	return nil
}

// Written returns the sorted paths written (or planned, in dry-run) by this txn.
func (t *Txn) Written() []string { return sortedKeys(t.written) }

func subject(command string, ids []string, counts string) string {
	s := "slowhunch: " + command
	if len(ids) > 5 {
		s += " " + strings.Join(ids[:5], " ") + fmt.Sprintf(" +%d more", len(ids)-5)
	} else if len(ids) > 0 {
		s += " " + strings.Join(ids, " ")
	}
	if counts != "" {
		s += " (" + counts + ")"
	}
	return s
}

func (t *Txn) body() string {
	lines := []string{"run: " + t.runID}
	if len(t.o.PromptVersions) > 0 {
		keys := make([]string, 0, len(t.o.PromptVersions))
		for k := range t.o.PromptVersions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + t.o.PromptVersions[k]
		}
		lines = append(lines, "prompts: "+strings.Join(parts, ", "))
	}
	return strings.Join(lines, "\n")
}

// Commit ends the run: one commit of Written() ∪ Pending (unless disabled),
// cleared pending state and an audit line. It returns the commit hash or "".
func (t *Txn) Commit(ids []string, counts string) (string, error) {
	if err := t.usable(); err != nil {
		return "", err
	}
	if t.o.DryRun {
		t.done = true
		return "", nil
	}
	set := map[string]bool{}
	for p := range t.written {
		if !internal(p) {
			set[p] = true
		}
	}
	for p := range t.pending {
		if !internal(p) {
			set[p] = true
		}
	}
	paths := sortedKeys(set)
	hash := ""
	if t.commitEnabled() && len(paths) > 0 {
		h, err := t.o.Committer.Commit(paths, subject(t.command, ids, counts), t.body())
		if err != nil {
			return "", err
		}
		hash = h
	}
	t.pending = map[string]bool{}
	t.state.Pending = nil
	if err := t.v.SaveState(t.state); err != nil {
		return hash, err
	}
	t.done = true
	err := t.v.Audit(AuditEntry{
		Time:           t.o.Now().Format(time.RFC3339),
		RunID:          t.runID,
		Command:        t.command,
		Paths:          paths,
		PromptVersions: t.o.PromptVersions,
		Commit:         hash,
	})
	return hash, err
}
