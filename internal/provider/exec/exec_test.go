package execprov

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

const helperMarker = "slowhunch-exec-helper"

func TestMain(m *testing.M) {
	if len(os.Args) >= 3 && os.Args[1] == helperMarker {
		os.Exit(runHelper(os.Args[2], os.Args[3:]))
	}
	os.Exit(m.Run())
}

func reply(text string) int {
	b, _ := json.Marshal(map[string]any{"text": text, "tokens_in": 7, "tokens_out": 3})
	os.Stdout.Write(b)
	return 0
}

func runHelper(mode string, rest []string) int {
	switch mode {
	case "echo":
		var req struct {
			Role, User string
		}
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 9
		}
		return reply(req.Role + "|" + req.User)
	case "env":
		env := os.Environ()
		sort.Strings(env)
		return reply(strings.Join(env, "\n"))
	case "args":
		return reply(strings.Join(rest, "\n"))
	case "bad":
		fmt.Print("not json")
		return 0
	case "fail":
		fmt.Fprint(os.Stderr, "boom with test-key inside")
		return 3
	case "rawenv":
		env := os.Environ()
		sort.Strings(env)
		fmt.Print(strings.Join(env, "\n"))
		return 0
	case "rawargs":
		fmt.Print(strings.Join(rest, "\n"))
		return 0
	case "stdin":
		b, _ := io.ReadAll(os.Stdin)
		os.Stdout.Write(b)
		return 0
	case "big":
		os.Stdout.Write(make([]byte, 4096))
		return 0
	case "sleep":
		time.Sleep(5 * time.Second)
		return reply("late")
	}
	return 99
}

func helper(t *testing.T, args ...string) *Runner {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(vault.ProviderConfig{Name: "exec", Command: append([]string{self, helperMarker}, args...)})
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Runner)
}

func wantProviderErr(t *testing.T, err error) {
	t.Helper()
	if errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("want PROVIDER_ERROR, got %v", err)
	}
}

func TestExecRoundTrip(t *testing.T) {
	r := helper(t, "echo")
	resp, err := r.Complete(context.Background(), provider.Request{Role: "gate", Medium: "text", System: "sys", User: "hello", MaxTokens: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "gate|hello" || resp.TokensIn != 7 || resp.TokensOut != 3 {
		t.Fatalf("got %+v", resp)
	}
	p, err := provider.New(vault.ProviderConfig{Name: "exec", Command: r.Command})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(*Runner); !ok {
		t.Fatalf("registry returned %T", p)
	}
}

func TestExecScrubbedEnv(t *testing.T) {
	t.Setenv("SLOWHUNCH_API_KEY", "test-key")
	t.Setenv("OTHER_VAR", "x")
	r := helper(t, "env")
	r.APIKeyEnv = "SLOWHUNCH_API_KEY"
	resp, err := r.Complete(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PATH=" + os.Getenv("PATH"), "SLOWHUNCH_API_KEY=test-key"}
	sort.Strings(want)
	if resp.Text != strings.Join(want, "\n") {
		t.Fatalf("helper env:\n%s", resp.Text)
	}

	r.APIKeyEnv = "SLOWHUNCH_UNSET_KEY"
	resp, err = r.Complete(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "PATH="+os.Getenv("PATH") {
		t.Fatalf("helper env without key:\n%s", resp.Text)
	}
}

func TestExecNoShell(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	args := []string{"$(touch pwned)", ";echo hi", "`touch pwned2`", "*", "$HOME"}
	r := helper(t, append([]string{"args"}, args...)...)
	resp, err := r.Complete(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != strings.Join(args, "\n") {
		t.Fatalf("args not verbatim: %q", resp.Text)
	}
	for _, f := range []string{"pwned", "pwned2"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Fatalf("%s was created", f)
		}
	}
}

func TestExecBadOutput(t *testing.T) {
	_, err := helper(t, "bad").Complete(context.Background(), provider.Request{})
	wantProviderErr(t, err)
}

func TestExecFailExit(t *testing.T) {
	t.Setenv("SLOWHUNCH_API_KEY", "test-key")
	r := helper(t, "fail")
	r.APIKeyEnv = "SLOWHUNCH_API_KEY"
	_, err := r.Complete(context.Background(), provider.Request{})
	wantProviderErr(t, err)
	msg := err.Error()
	if !strings.Contains(msg, "3") || !strings.Contains(msg, "boom") {
		t.Fatalf("message lacks exit code or stderr: %s", msg)
	}
	if strings.Contains(msg, "test-key") || !strings.Contains(msg, "***") {
		t.Fatalf("key not redacted: %s", msg)
	}
}

func TestExecTimeout(t *testing.T) {
	r := helper(t, "sleep")
	r.Timeout = 200 * time.Millisecond
	start := time.Now()
	_, err := r.Complete(context.Background(), provider.Request{})
	wantProviderErr(t, err)
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("timeout took %s", d)
	}
}

func TestExecEmptyCommand(t *testing.T) {
	for _, c := range [][]string{nil, {}, {""}} {
		_, err := New(vault.ProviderConfig{Name: "exec", Command: c})
		if errs.CodeOf(err) != errs.BadInput {
			t.Fatalf("command %q: got %v", c, err)
		}
	}
}

func TestRunArgv(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := func(args ...string) []string { return append([]string{self, helperMarker}, args...) }
	ctx := context.Background()

	t.Run("stdout", func(t *testing.T) {
		out, err := RunArgv(ctx, argv("stdin"), []byte("hello input"), nil, 0, 1<<10)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "hello input" {
			t.Fatalf("got %q", out)
		}
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("SLOWHUNCH_LEAK", "1")
		out, err := RunArgv(ctx, argv("rawenv"), nil, []string{"EXTRA_ONE=a", "EXTRA_TWO=b"}, 0, 1<<16)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"PATH=" + os.Getenv("PATH"), "EXTRA_ONE=a", "EXTRA_TWO=b"}
		sort.Strings(want)
		if string(out) != strings.Join(want, "\n") {
			t.Fatalf("env:\n%s", out)
		}
		if strings.Contains(string(out), "SLOWHUNCH_LEAK") {
			t.Fatalf("parent variable leaked: %s", out)
		}
	})

	t.Run("noshell", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		args := []string{"a;echo hacked", "$(touch pwned)", "*"}
		out, err := RunArgv(ctx, argv(append([]string{"rawargs"}, args...)...), nil, nil, 0, 1<<10)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != strings.Join(args, "\n") {
			t.Fatalf("args not verbatim: %q", out)
		}
		if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
			t.Fatal("pwned was created")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		start := time.Now()
		_, err := RunArgv(ctx, argv("sleep"), nil, nil, 200*time.Millisecond, 1<<10)
		if err == nil || !strings.Contains(err.Error(), "timed out after 200ms") {
			t.Fatalf("got %v", err)
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Fatalf("timeout took %s", d)
		}
	})

	t.Run("exit", func(t *testing.T) {
		_, err := RunArgv(ctx, argv("fail"), nil, nil, 0, 1<<10)
		if err == nil || !strings.Contains(err.Error(), "code 3") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("got %v", err)
		}
		if errs.CodeOf(err) == errs.ProviderError || IsNotFound(err) {
			t.Fatalf("unexpected classification: %v", err)
		}
	})

	t.Run("cap", func(t *testing.T) {
		_, err := RunArgv(ctx, argv("big"), nil, nil, 0, 100)
		if err == nil || !strings.Contains(err.Error(), "output exceeds") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("notfound", func(t *testing.T) {
		_, err := RunArgv(ctx, []string{"slowhunch-no-such-program-xyz"}, nil, nil, 0, 1<<10)
		if !IsNotFound(err) {
			t.Fatalf("lookup: got %v", err)
		}
		_, err = RunArgv(ctx, []string{filepath.Join(t.TempDir(), "missing")}, nil, nil, 0, 1<<10)
		if !IsNotFound(err) {
			t.Fatalf("path: got %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		for _, a := range [][]string{nil, {}, {""}} {
			_, err := RunArgv(ctx, a, nil, nil, 0, 1<<10)
			if errs.CodeOf(err) != errs.BadInput {
				t.Fatalf("argv %q: got %v", a, err)
			}
		}
	})
}
