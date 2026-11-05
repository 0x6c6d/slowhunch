package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

type ordered struct {
	Zeta  int    `json:"zeta"`
	Alpha string `json:"alpha"`
}

func init() {
	register("t ok", "test", func(env *Env, args []string) (any, error) {
		return ordered{1, "a"}, nil
	})
	register("t nil", "test", func(env *Env, args []string) (any, error) { return nil, nil })
	register("t err", "test", func(env *Env, args []string) (any, error) {
		code := ""
		if len(args) > 0 {
			code = args[0]
		}
		switch code {
		case "plain":
			return nil, os.ErrClosed
		case "fatal":
			return nil, errs.Fatal(errs.NotFound, "m", "h")
		}
		e := errs.New(code, "msg", "hint")
		e.Extra = map[string]any{"b": 1, "a": 2}
		return nil, e
	})
	register("t flags", "test", func(env *Env, args []string) (any, error) {
		fs := newFlagSet("t")
		as := fs.String("as", "", "")
		pos, err := parseFlags(fs, args)
		if err != nil {
			return nil, err
		}
		return map[string]any{"as": *as, "pos": pos, "json": env.JSON, "dry": env.DryRun,
			"nocommit": env.NoCommit, "vault": env.VaultFlag}, nil
	})
	register("t vault", "test", func(env *Env, args []string) (any, error) {
		v, err := env.VaultRoot()
		if err != nil {
			return nil, err
		}
		return map[string]any{"vault": v}, nil
	})
}

func run(args ...string) (int, string, string) {
	var o, e bytes.Buffer
	rc := Run(args, strings.NewReader(""), &o, &e)
	return rc, o.String(), e.String()
}

func TestSchema(t *testing.T) {
	rc, out, _ := run("t", "ok", "--json")
	if rc != 0 || !strings.HasPrefix(out, "{\n  \"schema\": 1,\n  \"ok\": true,\n  \"zeta\": 1,\n  \"alpha\": \"a\"\n}") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	_, out, _ = run("t", "nil", "--json")
	if out != "{\n  \"schema\": 1,\n  \"ok\": true\n}\n" {
		t.Fatalf("%q", out)
	}
	rc, out, _ = run("t", "err", errs.Exists, "--json")
	var v struct {
		Schema int
		Ok     bool
		Error  map[string]string
		A, B   int
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil || rc != 4 || v.Schema != 1 || v.Ok ||
		v.Error["code"] != "EXISTS" || v.Error["message"] != "msg" || v.Error["hint"] != "hint" || v.A != 2 || v.B != 1 {
		t.Fatalf("rc=%d out=%q err=%v", rc, out, err)
	}
	if strings.Index(out, `"a"`) > strings.Index(out, `"b"`) {
		t.Fatal("extra not sorted")
	}
	rc, _, errOut := run("t", "err", errs.Exists)
	if rc != 4 || !strings.Contains(errOut, "error: EXISTS: msg") || !strings.Contains(errOut, "hint: hint") {
		t.Fatalf("%q", errOut)
	}
}

func TestExitCodes(t *testing.T) {
	if rc, _, _ := run("nosuch"); rc != 2 {
		t.Fatal(rc)
	}
	if rc, _, _ := run(); rc != 2 {
		t.Fatal(rc)
	}
	want := map[string]int{
		errs.Conflict: 4, errs.ConflictRev: 4, errs.Exists: 4,
		errs.NotFound: 2, errs.BadInput: 2, errs.Unsupported: 2, errs.TooLong: 2,
		errs.LintFailed: 3, errs.GitMissing: 1, errs.ProviderError: 1,
	}
	for code, w := range want {
		if rc, _, _ := run("t", "err", code); rc != w {
			t.Errorf("%s: %d want %d", code, rc, w)
		}
	}
	if rc, _, _ := run("t", "err", "fatal"); rc != 1 {
		t.Error("fatal")
	}
	rc, out, _ := run("t", "err", "plain", "--json")
	if rc != 1 || !strings.Contains(out, "internal error") || !strings.Contains(out, "BAD_INPUT") {
		t.Errorf("plain: %d %q", rc, out)
	}
	if rc, out, _ := run("help"); rc != 0 || !strings.Contains(out, "t flags") {
		t.Error("help")
	}
}

func TestGlobalFlags(t *testing.T) {
	_, out, _ := run("--json", "--dry-run", "t", "flags", "x", "--as", "poem", "y", "--no-commit", "--vault=v1")
	var v struct {
		As       string
		Pos      []string
		JSON     bool
		Dry      bool
		Nocommit bool
		Vault    string
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err, out)
	}
	if v.As != "poem" || strings.Join(v.Pos, ",") != "x,y" || !v.JSON || !v.Dry || !v.Nocommit || v.Vault != "v1" {
		t.Fatalf("%+v", v)
	}
	_, out, _ = run("--vault", "v2", "t", "flags", "--json", "--", "--as", "z")
	v.Pos = nil
	if err := json.Unmarshal([]byte(out), &v); err != nil || v.Vault != "v2" || strings.Join(v.Pos, ",") != "--as,z" {
		t.Fatalf("%+v %v", v, err)
	}
	if rc, _, _ := run("t", "flags", "--bogus"); rc != 2 {
		t.Fatal(rc)
	}
}

func TestVaultLookup(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if err := os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLOWHUNCH_VAULT", "")
	get := func(args ...string) (int, string) {
		rc, out, _ := run(append([]string{"t", "vault", "--json"}, args...)...)
		var v struct{ Vault string }
		json.Unmarshal([]byte(out), &v)
		if rc != 0 {
			return rc, out
		}
		return rc, v.Vault
	}
	if rc, v := get(); rc != 0 || v != root {
		t.Fatalf("parent: %d %q", rc, v)
	}
	envDir := filepath.Join(t.TempDir(), "env")
	t.Setenv("SLOWHUNCH_VAULT", envDir)
	if _, v := get(); v != envDir {
		t.Fatalf("env: %q", v)
	}
	flagDir := filepath.Join(t.TempDir(), "flag")
	if _, v := get("--vault", flagDir); v != flagDir {
		t.Fatalf("flag: %q", v)
	}
	t.Setenv("SLOWHUNCH_VAULT", "")
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	rc, out := get()
	if rc != 2 || !strings.Contains(out, "NOT_FOUND") {
		t.Fatalf("none: %d %q", rc, out)
	}
}
