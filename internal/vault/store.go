package vault

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Vault is an opened vault; every write goes through its methods.
type Vault struct {
	Root string // absolute, symlinks resolved
}

// Open opens the vault at root, which must contain a .slowhunch directory.
func Open(root string) (*Vault, error) {
	if err := GuardPath(root); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, errs.New(errs.BadInput, "bad vault path: "+root, "")
	}
	r, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, errs.New(errs.NotFound, "no vault at "+root, "run `slowhunch init` first")
	}
	fi, err := os.Stat(filepath.Join(r, ".slowhunch"))
	if err != nil || !fi.IsDir() {
		return nil, errs.New(errs.NotFound, "no vault at "+root+" (missing .slowhunch/)", "run `slowhunch init` first")
	}
	return &Vault{Root: r}, nil
}

func refuse(rel, why string) error {
	return errs.New(errs.BadInput, "refusing path "+rel+": "+why, "use a relative path inside the vault")
}

// clean validates rel lexically and returns its cleaned slash form.
func clean(rel string) (string, error) {
	if rel == "" {
		return "", refuse(rel, "empty path")
	}
	if strings.IndexByte(rel, 0) >= 0 {
		return "", refuse(rel, "NUL byte")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", refuse(rel, "absolute path")
	}
	for _, el := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if el == ".." {
			return "", refuse(rel, "'..' element")
		}
	}
	c := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if c == "." {
		return "", refuse(rel, "vault root")
	}
	if c == ".git" || strings.HasPrefix(c, ".git/") {
		return "", refuse(rel, "inside .git/")
	}
	return c, nil
}

// Abs confines rel to the vault and returns the absolute, resolved path.
func (v *Vault) Abs(rel string) (string, error) {
	c, err := clean(rel)
	if err != nil {
		return "", err
	}
	r, err := resolve(filepath.Join(v.Root, filepath.FromSlash(c)))
	if err != nil {
		return "", refuse(rel, "cannot resolve symlinks")
	}
	if r == v.Root || !within(r, v.Root) {
		return "", refuse(rel, "resolves outside the vault")
	}
	inner, _ := filepath.Rel(v.Root, r)
	inner = filepath.ToSlash(inner)
	if inner == ".git" || strings.HasPrefix(inner, ".git/") {
		return "", refuse(rel, "inside .git/")
	}
	return r, nil
}

// Exists reports whether rel exists inside the vault.
func (v *Vault) Exists(rel string) bool {
	p, err := v.Abs(rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// ReadFile reads rel; NOT_FOUND if it is missing.
func (v *Vault) ReadFile(rel string) ([]byte, error) {
	p, err := v.Abs(rel)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errs.New(errs.NotFound, "not found: "+rel, "")
	}
	return b, err
}

// mkdirs creates the parent directories of rel one by one, each confined.
func (v *Vault) mkdirs(rel string) error {
	c, err := clean(rel)
	if err != nil {
		return err
	}
	parts := strings.Split(c, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		p, err := v.Abs(dir)
		if err != nil {
			return err
		}
		if err := os.Mkdir(p, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			return refuse(rel, dir+" is not a directory")
		}
	}
	return nil
}

func isInbox(rel string) bool {
	c, err := clean(rel)
	return err == nil && strings.HasPrefix(c, "inbox/")
}

// WriteFile atomically writes data to rel. Existing inbox files are immutable.
func (v *Vault) WriteFile(rel string, data []byte) error {
	if _, err := v.Abs(rel); err != nil {
		return err
	}
	if isInbox(rel) && v.Exists(rel) {
		return errs.New(errs.Exists, "inbox files are immutable: "+rel, "")
	}
	if err := v.mkdirs(rel); err != nil {
		return err
	}
	p, err := v.Abs(rel)
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	f, err := os.CreateTemp(dir, "."+filepath.Base(p)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	ok = true
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// Move renames from to to; the target must not exist.
func (v *Vault) Move(from, to string) error {
	src, err := v.Abs(from)
	if err != nil {
		return err
	}
	if _, err := v.Abs(to); err != nil {
		return err
	}
	if _, err := os.Lstat(src); err != nil {
		return errs.New(errs.NotFound, "not found: "+from, "")
	}
	if v.Exists(to) {
		return errs.New(errs.Exists, "target exists: "+to, "")
	}
	if err := v.mkdirs(to); err != nil {
		return err
	}
	dst, err := v.Abs(to)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return errs.New(errs.Exists, "target exists: "+to, "")
	}
	return os.Rename(src, dst)
}

// Remove deletes rel; NOT_FOUND if it is missing.
func (v *Vault) Remove(rel string) error {
	p, err := v.Abs(rel)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, fs.ErrNotExist) {
		return errs.New(errs.NotFound, "not found: "+rel, "")
	}
	return err
}

// PutDoc writes d to rel, guarded by expectRev, and returns the new rev.
func (v *Vault) PutDoc(rel string, d *frontmatter.Doc, expectRev string) (string, error) {
	if _, err := v.Abs(rel); err != nil {
		return "", err
	}
	if v.Exists(rel) {
		cur, err := v.ReadFile(rel)
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
	if err := v.WriteFile(rel, d.Bytes()); err != nil {
		return "", err
	}
	return rev, nil
}

// AuditEntry is one line of .slowhunch/audit.log. It holds paths, never contents.
type AuditEntry struct {
	Time           string            `json:"time"`
	RunID          string            `json:"run_id"`
	Command        string            `json:"command"`
	Paths          []string          `json:"paths"`
	PromptVersions map[string]string `json:"prompt_versions"`
	Commit         string            `json:"commit"`
}

// Audit appends one JSON line to .slowhunch/audit.log.
func (v *Vault) Audit(e AuditEntry) error {
	p, err := v.Abs(".slowhunch/audit.log")
	if err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// State is the persisted run state in .slowhunch/state.json.
type State struct {
	PromptVersions map[string]string `json:"prompt_versions"`
	Pending        []string          `json:"pending"`
	// RunsByDay counts runs by date (YYYY-MM-DD) and medium.
	RunsByDay map[string]map[string]int `json:"runs_by_day,omitempty"`
}

// LoadState reads .slowhunch/state.json; a missing file yields a zero State.
func (v *Vault) LoadState() (State, error) {
	var s State
	b, err := v.ReadFile(".slowhunch/state.json")
	if errs.CodeOf(err) == errs.NotFound {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, errs.New(errs.BadInput, "invalid .slowhunch/state.json: "+err.Error(), "")
	}
	return s, nil
}

// SaveState atomically writes .slowhunch/state.json.
func (v *Vault) SaveState(s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return v.WriteFile(".slowhunch/state.json", append(b, '\n'))
}
