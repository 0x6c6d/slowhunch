package vault

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

// resolve returns the absolute, symlink-resolved form of path. Missing trailing
// components are kept as-is after the deepest existing ancestor is resolved.
func resolve(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		if _, err := os.Lstat(p); err == nil {
			r, err := filepath.EvalSymlinks(p)
			if err != nil {
				return "", err
			}
			for i := len(tail) - 1; i >= 0; i-- {
				r = filepath.Join(r, tail[i])
			}
			return r, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path, nil
		}
		tail = append(tail, filepath.Base(p))
		p = parent
	}
}

// within reports whether p equals dir or lies inside it.
func within(p, dir string) bool {
	if p == dir {
		return true
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	return strings.HasPrefix(p, dir)
}

// GuardPath refuses paths equal to or inside a protected directory while
// SLOWHUNCH_TEST=1, so tests can never touch a real vault.
func GuardPath(path string) error {
	if os.Getenv("SLOWHUNCH_TEST") != "1" {
		return nil
	}
	prot := os.Getenv("SLOWHUNCH_PROTECTED")
	if prot == "" {
		return nil
	}
	rp, err := resolve(path)
	if err != nil {
		return errs.New(errs.BadInput, "refusing protected path", "cannot resolve "+path)
	}
	for _, d := range filepath.SplitList(prot) {
		if d == "" {
			continue
		}
		rd, err := resolve(d)
		if err != nil {
			rd = filepath.Clean(d)
		}
		if within(rp, rd) {
			return errs.New(errs.BadInput, "refusing protected path",
				"SLOWHUNCH_TEST=1 forbids touching "+d)
		}
	}
	return nil
}
