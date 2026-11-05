// Package execprov implements the exec provider: it runs a local program
// directly (no shell) with a scrubbed environment and exchanges JSON over
// stdin and stdout.
package execprov

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

const (
	defaultTimeout = 120 * time.Second
	stdoutCap      = 1 << 20
	stderrCap      = 4 << 10
	stderrSnippet  = 200
)

// Runner runs a local command as a provider.
type Runner struct {
	Command   []string
	APIKeyEnv string
	Timeout   time.Duration // default 120 s
}

func init() { provider.Register("exec", New) }

// New builds a Runner from cfg; cfg.Command must name a program.
func New(cfg vault.ProviderConfig) (provider.Provider, error) {
	if len(cfg.Command) == 0 || cfg.Command[0] == "" {
		return nil, errs.Fatal(errs.BadInput, "command is required for the exec provider", "set provider.command to the program and its arguments")
	}
	return &Runner{
		Command:   append([]string(nil), cfg.Command...),
		APIKeyEnv: cfg.APIKeyEnv,
		Timeout:   defaultTimeout,
	}, nil
}

type execRequest struct {
	Role      string `json:"role"`
	Medium    string `json:"medium"`
	System    string `json:"system"`
	User      string `json:"user"`
	MaxTokens int    `json:"max_tokens"`
}

type execResponse struct {
	Text      *string `json:"text"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
}

// capWriter keeps at most limit bytes and records whether more arrived.
type capWriter struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	room := w.limit - w.buf.Len()
	if room < len(p) {
		w.overflow = true
		if room > 0 {
			w.buf.Write(p[:room])
		}
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func redact(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "***")
}

func providerErr(msg, key string) error {
	return errs.New(errs.ProviderError, redact(msg, key), "")
}

// exitError is a non-zero exit of a program run by RunArgv. It keeps the
// captured stderr so Complete can redact before truncating, exactly as before.
type exitError struct {
	code   int
	stderr string
	err    error
}

func (e *exitError) Error() string { return exitMessage(e.code, e.stderr) }

func (e *exitError) Unwrap() error { return e.err }

// timeoutError reports that RunArgv hit its deadline.
type timeoutError struct{ d time.Duration }

func (e *timeoutError) Error() string { return fmt.Sprintf("command timed out after %s", e.d) }

func (e *timeoutError) Unwrap() error { return context.DeadlineExceeded }

func exitMessage(code int, stderr string) string {
	if len(stderr) > stderrSnippet {
		stderr = stderr[:stderrSnippet]
	}
	return fmt.Sprintf("command exited with code %d: %s", code, strings.TrimSpace(stderr))
}

func capText(n int) string {
	if n > 0 && n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n/(1<<20))
	}
	return fmt.Sprintf("%d bytes", n)
}

// IsNotFound reports whether err from RunArgv means the program does not exist.
func IsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
}

// RunArgv runs argv[0] with argv[1:] directly (no shell), with stdin as standard input and an
// environment of PATH plus the extra "KEY=value" entries only. It returns stdout (capped) and an error.
func RunArgv(ctx context.Context, argv []string, stdin []byte, extraEnv []string, timeout time.Duration, stdoutCap int) ([]byte, error) {
	if len(argv) == 0 || argv[0] == "" {
		return nil, errs.Fatal(errs.BadInput, "a program to run is required", "")
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	env := append([]string{"PATH=" + os.Getenv("PATH")}, extraEnv...)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := &capWriter{limit: stdoutCap}
	stderr := &capWriter{limit: stderrCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second

	runErr := cmd.Run()
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, &timeoutError{d: timeout}
		}
		return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return nil, &exitError{code: ee.ExitCode(), stderr: stderr.buf.String(), err: runErr}
		}
		return nil, fmt.Errorf("run command: %w", runErr)
	}
	if stdout.overflow {
		return nil, fmt.Errorf("command output exceeds %s", capText(stdoutCap))
	}
	return stdout.buf.Bytes(), nil
}

// Complete runs the command once with req as JSON on stdin.
func (r *Runner) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	if len(r.Command) == 0 || r.Command[0] == "" {
		return provider.Response{}, errs.Fatal(errs.BadInput, "command is required for the exec provider", "")
	}
	key := ""
	var extra []string
	if r.APIKeyEnv != "" {
		key = os.Getenv(r.APIKeyEnv)
		if key != "" {
			extra = append(extra, r.APIKeyEnv+"="+key)
		}
	}
	input, err := json.Marshal(execRequest{
		Role: req.Role, Medium: req.Medium, System: req.System, User: req.User, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return provider.Response{}, providerErr("encode request: "+err.Error(), key)
	}
	out, err := RunArgv(ctx, r.Command, input, extra, r.Timeout, stdoutCap)
	if err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return provider.Response{}, providerErr(exitMessage(ee.code, redact(ee.stderr, key)), key)
		}
		return provider.Response{}, providerErr(err.Error(), key)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var resp execResponse
	if err := dec.Decode(&resp); err != nil {
		return provider.Response{}, providerErr("invalid command output JSON: "+err.Error(), key)
	}
	if _, err := dec.Token(); err != io.EOF {
		return provider.Response{}, providerErr("command output must be exactly one JSON object", key)
	}
	if resp.Text == nil {
		return provider.Response{}, providerErr("command output has no text field", key)
	}
	return provider.Response{Text: *resp.Text, TokensIn: resp.TokensIn, TokensOut: resp.TokensOut}, nil
}
