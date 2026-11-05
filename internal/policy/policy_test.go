package policy

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

const modPath = "github.com/0x6c6d/slowhunch"

// skipDirs are never walked.
var skipDirs = map[string]bool{".git": true, ".harness": true, ".claude": true, "bin": true}

type goFile struct {
	path string // absolute path
	rel  string // slash-separated, relative to the module root
	test bool   // *_test.go
}

type importRef struct {
	path string
	line int
}

// moduleRoot walks up from the test's working directory to the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// goFiles lists every .go file in the module, skipping skipDirs.
func goFiles(t *testing.T) []goFile {
	t.Helper()
	root := moduleRoot(t)
	var files []goFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, goFile{
			path: p,
			rel:  filepath.ToSlash(rel),
			test: strings.HasSuffix(p, "_test.go"),
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// importsOf parses only the import block of f.
func importsOf(t *testing.T, f goFile) []importRef {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, f.path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", f.rel, err)
	}
	var out []importRef
	for _, spec := range af.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: bad import literal %s", f.rel, spec.Path.Value)
		}
		out = append(out, importRef{path: p, line: fset.Position(spec.Pos()).Line})
	}
	return out
}

var forbiddenRe = regexp.MustCompile(`git (push|pull)|--force`)

var forbiddenLits = map[string]bool{"push": true, "pull": true, "--force": true}

func TestNoForbiddenGit(t *testing.T) {
	for _, f := range goFiles(t) {
		if f.test {
			continue
		}
		src, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if m := forbiddenRe.FindString(line); m != "" {
				t.Errorf("%s:%d: forbidden text %q", f.rel, i+1, m)
			}
		}
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, f.path, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", f.rel, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && forbiddenLits[v] {
				t.Errorf("%s:%d: forbidden string literal %s", f.rel, fset.Position(lit.Pos()).Line, lit.Value)
			}
			return true
		})
	}
}

// checkImportOnly fails for any non-test file importing path outside the allowed prefixes.
func checkImportOnly(t *testing.T, path string, allowed []string) {
	t.Helper()
	for _, f := range goFiles(t) {
		if f.test {
			continue
		}
		ok := false
		for _, a := range allowed {
			if strings.HasPrefix(f.rel, a) {
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		for _, im := range importsOf(t, f) {
			if im.path == path {
				t.Errorf("%s:%d imports %q; only %v may", f.rel, im.line, path, allowed)
			}
		}
	}
}

func TestNetworkOnlyOpenAI(t *testing.T) {
	checkImportOnly(t, "net/http", []string{"internal/provider/openai/"})
}

func TestExecOnlyGitopsAndExecProvider(t *testing.T) {
	checkImportOnly(t, "os/exec", []string{"internal/gitops/", "internal/provider/exec/"})
}

func TestStdlibOnly(t *testing.T) {
	root := moduleRoot(t)
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(gomod), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "require" {
			t.Errorf("go.mod:%d: require directive %q", i+1, strings.TrimSpace(line))
		}
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Errorf("go.sum exists")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	for _, f := range goFiles(t) {
		for _, im := range importsOf(t, f) {
			first, _, _ := strings.Cut(im.path, "/")
			if !strings.Contains(first, ".") {
				continue // standard library
			}
			if im.path == modPath || strings.HasPrefix(im.path, modPath+"/") {
				continue
			}
			t.Errorf("%s:%d: non-stdlib import %q", f.rel, im.line, im.path)
		}
	}
}

func TestTestGuard(t *testing.T) {
	t.Setenv("SLOWHUNCH_TEST", "1")
	a := t.TempDir()
	b := t.TempDir()
	t.Setenv("SLOWHUNCH_PROTECTED", a)

	for _, p := range []string{a, filepath.Join(a, "sub", "v")} {
		if err := vault.GuardPath(p); err == nil {
			t.Errorf("GuardPath(%q) allowed a protected path", p)
		}
	}
	if err := vault.GuardPath(b); err != nil {
		t.Errorf("GuardPath(%q) refused a sibling directory: %v", b, err)
	}

	inside := filepath.Join(a, "v")
	if err := os.MkdirAll(filepath.Join(inside, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Open(inside); err == nil {
		t.Errorf("vault.Open(%q) succeeded inside a protected directory", inside)
	}

	// Control: a vault outside the protected directory still opens.
	if err := os.Mkdir(filepath.Join(b, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Open(b); err != nil {
		t.Errorf("vault.Open(%q) failed outside the protected directory: %v", b, err)
	}
}
