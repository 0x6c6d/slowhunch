// Package gitops is the only place in slowhunch that runs git. Every call
// uses an argv slice (never a shell) and puts "--" before user paths.
package gitops

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

// netTimeout bounds every network-touching git call (pull, push).
const netTimeout = 120 * time.Second

// firstLine returns the first non-empty trimmed line of s.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// runNet executes git -C dir <args...> with GIT_TERMINAL_PROMPT=0 and a
// timeout, so it never waits for credentials. On failure it returns the
// first stderr line (or the process error) as msg and a non-nil err.
func runNet(dir string, args ...string) (msg string, err error) {
	if !Available() {
		return "", missing()
	}
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	argv := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		line := firstLine(stderr.String())
		if ctx.Err() != nil {
			line = "git timed out after " + netTimeout.String()
		}
		if line == "" {
			line = err.Error()
		}
		return line, err
	}
	return "", nil
}

// HasRemote reports whether a remote called name is configured.
func (r *Repo) HasRemote(name string) (bool, error) {
	out, err := run(r.Dir, "remote", "remote")
	if err != nil {
		return false, err
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) == name && name != "" {
			return true, nil
		}
	}
	return false, nil
}

// CurrentBranch returns the short name of the checked-out branch.
func (r *Repo) CurrentBranch() (string, error) {
	out, err := run(r.Dir, "symbolic-ref", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// HasUpstream reports whether the current branch has an upstream.
func (r *Repo) HasUpstream() (bool, error) {
	if !Available() {
		return false, missing()
	}
	out, err := run(r.Dir, "rev-parse", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// Head returns the full hash of HEAD.
func (r *Repo) Head() (string, error) {
	out, err := run(r.Dir, "rev-parse", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repo) porcelain() (string, error) {
	out, err := run(r.Dir, "status", "status", "--porcelain")
	return string(out), err
}

func (r *Repo) count(rng string) (int, error) {
	out, err := run(r.Dir, "rev-list", "rev-list", "--count", rng)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, errs.Fatal(errs.BadInput, "git rev-list: unexpected output", "")
	}
	return n, nil
}

// PullRebase runs git pull --rebase --autostash and returns how many new
// commits arrived (commits now in HEAD's history that the old HEAD lacked,
// not counting local commits replayed on top). It never creates a merge commit and never resolves a
// conflict: on conflict it aborts the rebase and checks HEAD and the work
// tree are back to where they were.
func (r *Repo) PullRebase() (pulled int, err error) {
	if !Available() {
		return 0, missing()
	}
	oldHead, err := r.Head()
	if err != nil {
		return 0, err
	}
	oldStatus, err := r.porcelain()
	if err != nil {
		return 0, err
	}
	args := append(r.identity(), "pull", "--rebase", "--autostash", "-q")
	msg, perr := runNet(r.Dir, args...)
	if perr != nil {
		if e, ok := perr.(*errs.Error); ok {
			return 0, e
		}
		out, _ := run(r.Dir, "diff", "diff", "--name-only", "--diff-filter=U")
		var files []string
		for _, l := range strings.Split(string(out), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				files = append(files, l)
			}
		}
		if len(files) == 0 {
			return 0, errs.New(errs.PullFailed, msg, "check git.remote and your network")
		}
		sort.Strings(files)
		text := "pull conflicts in " + strings.Join(files, ", ")
		_, aerr := run(r.Dir, "rebase", append(r.identity(), "rebase", "--abort")...)
		head, herr := r.Head()
		status, serr := r.porcelain()
		switch {
		case aerr != nil:
			text += "; rebase --abort failed (" + aerr.Error() + "), the work tree may not be restored"
		case herr != nil || serr != nil || head != oldHead || status != oldStatus:
			text += "; could not restore HEAD and the work tree to their state before the pull"
		}
		e := errs.New(errs.Conflict, text, "resolve the conflict with git yourself, then run again")
		e.Extra = map[string]any{"paths": files}
		return 0, e
	}
	newHead, err := r.Head()
	if err != nil {
		return 0, err
	}
	if newHead == oldHead {
		return 0, nil
	}
	// Local commits replayed by the rebase get new hashes, so count the
	// upstream commits the old HEAD lacked; old..new only without upstream.
	if ok, _ := r.HasUpstream(); ok {
		return r.count(oldHead + "..@{u}")
	}
	return r.count(oldHead + ".." + newHead)
}

// validRef rejects empty names and names that git could read as an
// option or a force refspec.
func validRef(kind, v string) error {
	if v == "" || strings.HasPrefix(v, "-") || strings.HasPrefix(v, "+") {
		return errs.New(errs.BadInput, "invalid push "+kind+" "+strconv.Quote(v), "use a plain remote and branch name")
	}
	return nil
}

// Push runs exactly git push <remote> <branch>.
func (r *Repo) Push(remote, branch string) error {
	if err := validRef("remote", remote); err != nil {
		return err
	}
	if err := validRef("branch", branch); err != nil {
		return err
	}
	msg, err := runNet(r.Dir, "push", remote, branch)
	if err != nil {
		if e, ok := err.(*errs.Error); ok {
			return e
		}
		return errs.New(errs.PushFailed, msg, "the commit is kept; the next run that pushes sends it")
	}
	return nil
}

// Behind returns how many upstream commits HEAD lacks (as of the last
// fetch); 0 when the branch has no upstream.
func (r *Repo) Behind() (int, error) {
	ok, err := r.HasUpstream()
	if err != nil || !ok {
		return 0, err
	}
	return r.count("HEAD..@{u}")
}
