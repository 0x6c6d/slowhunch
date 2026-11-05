// Package cli implements command dispatch, global flags and output envelopes.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

var nowFunc = time.Now

// Env is the per-invocation environment passed to handlers.
type Env struct {
	Stdin                  io.Reader
	Stdout, Stderr         io.Writer
	JSON, DryRun, NoCommit bool
	VaultFlag              string
	Now                    func() time.Time
}

// Handler runs a command and returns a result to render.
type Handler func(env *Env, args []string) (any, error)

type command struct {
	summary string
	h       Handler
}

var commands = map[string]command{}

func register(name, summary string, h Handler) {
	commands[name] = command{summary, h}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseFlags parses flags interleaved with positionals; "--" ends flags.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var tail []string
	for i, a := range args {
		if a == "--" {
			tail = args[i+1:]
			args = args[:i]
			break
		}
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, errs.New(errs.BadInput, err.Error(), "see slowhunch help")
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
	return append(pos, tail...), nil
}

// VaultRoot resolves the vault directory.
func (e *Env) VaultRoot() (string, error) {
	dir := e.VaultFlag
	if dir == "" {
		dir = os.Getenv("SLOWHUNCH_VAULT")
	}
	if dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", errs.New(errs.BadInput, err.Error(), "")
		}
		return abs, nil
	}
	wd, err := os.Getwd()
	if err == nil {
		for d := wd; ; d = filepath.Dir(d) {
			if st, err := os.Stat(filepath.Join(d, ".slowhunch")); err == nil && st.IsDir() {
				return d, nil
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	return "", errs.New(errs.NotFound, "no vault found", "run slowhunch init <dir> or pass --vault")
}

// Run executes the CLI and returns the exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	env := &Env{Stdin: stdin, Stdout: stdout, Stderr: stderr, Now: nowFunc}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			rest = append(rest, args[i:]...)
			i = len(args)
		case a == "--json":
			env.JSON = true
		case a == "--dry-run":
			env.DryRun = true
		case a == "--no-commit":
			env.NoCommit = true
		case a == "--vault":
			if i+1 >= len(args) {
				return fail(env, errs.New(errs.BadInput, "--vault needs a directory", "usage: --vault <dir>"), true)
			}
			i++
			env.VaultFlag = args[i]
		case strings.HasPrefix(a, "--vault="):
			env.VaultFlag = strings.TrimPrefix(a, "--vault=")
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return fail(env, errs.New(errs.BadInput, "no command given", "run slowhunch help"), true)
	}
	if rest[0] == "help" {
		fmt.Fprint(stdout, helpText())
		return 0
	}
	var cmd command
	var cargs []string
	found := false
	if len(rest) >= 2 {
		if c, ok := commands[rest[0]+" "+rest[1]]; ok {
			cmd, cargs, found = c, rest[2:], true
		}
	}
	if !found {
		if c, ok := commands[rest[0]]; ok {
			cmd, cargs, found = c, rest[1:], true
		}
	}
	if !found {
		return fail(env, errs.New(errs.BadInput, "unknown command: "+rest[0], "run slowhunch help"), true)
	}
	res, err := cmd.h(env, cargs)
	if err != nil {
		return fail(env, err, false)
	}
	if err := printResult(env, res); err != nil {
		return fail(env, err, false)
	}
	return 0
}

func helpText() string {
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("usage: slowhunch [--json] [--dry-run] [--no-commit] [--vault <dir>] <command> [args]\n\ncommands:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "  %-20s %s\n", n, commands[n].summary)
	}
	b.WriteString("  help                 show this list\n")
	return b.String()
}

func printResult(env *Env, res any) error {
	if env.JSON {
		inner := ""
		if res != nil {
			raw, err := json.Marshal(res)
			if err != nil {
				return err
			}
			raw = bytes.TrimSpace(raw)
			if len(raw) >= 2 && raw[0] == '{' && raw[len(raw)-1] == '}' {
				inner = string(bytes.TrimSpace(raw[1 : len(raw)-1]))
			} else if string(raw) != "null" {
				return errors.New("handler result is not a JSON object")
			}
		}
		return writeEnvelope(env.Stdout, `"schema": 1, "ok": true`, inner)
	}
	if t, ok := res.(interface{ Text() string }); ok {
		s := t.Text()
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		_, err := io.WriteString(env.Stdout, s)
		return err
	}
	if res == nil {
		return nil
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err = env.Stdout.Write(buf.Bytes())
	return err
}

func writeEnvelope(w io.Writer, head, inner string) error {
	s := "{" + head
	if inner != "" {
		s += ", " + inner
	}
	s += "}"
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

func fail(env *Env, err error, usage bool) int {
	var e *errs.Error
	if !errors.As(err, &e) {
		e = &errs.Error{Code: errs.BadInput, Message: err.Error(), Hint: "internal error", Exit: 1}
	}
	code := errs.ExitCode(e)
	if env.JSON {
		q := func(v any) string { b, _ := json.Marshal(v); return string(b) }
		obj := `"code": ` + q(e.Code) + `, "message": ` + q(e.Message) + `, "hint": ` + q(e.Hint)
		inner := `"error": {` + obj + `}`
		keys := make([]string, 0, len(e.Extra))
		for k := range e.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			inner += ", " + q(k) + ": " + q(e.Extra[k])
		}
		_ = writeEnvelope(env.Stdout, `"schema": 1, "ok": false`, inner)
		return code
	}
	fmt.Fprintf(env.Stderr, "error: %s\n", e.Error())
	if e.Hint != "" {
		fmt.Fprintf(env.Stderr, "hint: %s\n", e.Hint)
	}
	if usage {
		fmt.Fprint(env.Stderr, "usage: slowhunch [--json] [--dry-run] [--no-commit] [--vault <dir>] <command> [args]\n")
	}
	return code
}
