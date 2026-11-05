// Package gitops is the only place in slowhunch that runs git. Every call
// uses an argv slice (never a shell) and puts "--" before user paths.
package gitops

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

func missing() error {
	return errs.Fatal(errs.GitMissing, "git not found", "install git or use --no-commit")
}

// Available reports whether git is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// run executes git -C dir <args...> and returns stdout. sub names the
// subcommand for error messages.
func run(dir, sub string, args ...string) ([]byte, error) {
	if !Available() {
		return nil, missing()
	}
	argv := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", argv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		line := strings.TrimSpace(stderr.String())
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			line = err.Error()
		}
		return nil, errs.Fatal(errs.BadInput, "git "+sub+" failed: "+line, "")
	}
	return stdout.Bytes(), nil
}

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(dir string) bool {
	out, err := run(dir, "rev-parse", "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// Init runs git init -q in dir.
func Init(dir string) error {
	_, err := run(dir, "init", "init", "-q")
	return err
}

// Repo is a git work tree (or a directory inside one).
type Repo struct{ Dir string }

// New returns a Repo rooted at dir.
func New(dir string) *Repo { return &Repo{Dir: dir} }

func cleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		c := filepath.ToSlash(filepath.Clean(p))
		if p == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// Dirty returns (sorted) those of paths (relative to Dir) that have
// uncommitted changes: modified, staged, deleted or untracked-but-present.
func (r *Repo) Dirty(paths []string) ([]string, error) {
	if !Available() {
		return nil, missing()
	}
	ps := cleanPaths(paths)
	if len(ps) == 0 {
		return nil, nil
	}
	pre, err := run(r.Dir, "rev-parse", "rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSpace(string(pre))
	args := append([]string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--"}, ps...)
	out, err := run(r.Dir, "status", args...)
	if err != nil {
		return nil, err
	}
	var changed []string
	toks := strings.Split(string(out), "\x00")
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if len(t) < 4 {
			continue
		}
		xy, p := t[:2], t[3:]
		if xy == "!!" {
			continue
		}
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // skip the original path of a rename/copy
		}
		changed = append(changed, strings.TrimPrefix(p, prefix))
	}
	var dirty []string
	for _, p := range ps {
		for _, c := range changed {
			if c == p || strings.HasPrefix(c, p+"/") {
				dirty = append(dirty, p)
				break
			}
		}
	}
	sort.Strings(dirty)
	return dirty, nil
}

// identity returns the -c options that make a commit possible without
// touching any config.
func (r *Repo) identity() []string {
	opts := []string{"-c", "commit.gpgsign=false"}
	name, _ := run(r.Dir, "config", "config", "--get", "user.name")
	email, _ := run(r.Dir, "config", "config", "--get", "user.email")
	if strings.TrimSpace(string(name)) == "" || strings.TrimSpace(string(email)) == "" {
		opts = append(opts, "-c", "user.name=slowhunch", "-c", "user.email=slowhunch@localhost")
	}
	return opts
}

// Commit stages exactly paths and commits only them. It returns the full
// hash of the new HEAD, or "" when none of the paths changed.
func (r *Repo) Commit(paths []string, subject, body string) (string, error) {
	if !Available() {
		return "", missing()
	}
	dirty, err := r.Dirty(paths)
	if err != nil {
		return "", err
	}
	if len(dirty) == 0 {
		return "", nil
	}
	var present []string
	for _, p := range dirty {
		if _, err := os.Lstat(filepath.Join(r.Dir, filepath.FromSlash(p))); err == nil {
			present = append(present, p)
		} else {
			if _, err := run(r.Dir, "rm", "rm", "--cached", "-q", "--ignore-unmatch", "--", p); err != nil {
				return "", err
			}
		}
	}
	if len(present) > 0 {
		if _, err := run(r.Dir, "add", append([]string{"add", "--"}, present...)...); err != nil {
			return "", err
		}
	}
	args := append(r.identity(), "commit", "-q", "--only", "-m", subject)
	if body != "" {
		args = append(args, "-m", body)
	}
	args = append(append(args, "--"), dirty...)
	if _, err := run(r.Dir, "commit", args...); err != nil {
		return "", err
	}
	out, err := run(r.Dir, "rev-parse", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
