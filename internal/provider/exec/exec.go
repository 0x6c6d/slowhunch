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

// Complete runs the command once with req as JSON on stdin.
func (r *Runner) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	if len(r.Command) == 0 || r.Command[0] == "" {
		return provider.Response{}, errs.Fatal(errs.BadInput, "command is required for the exec provider", "")
	}
	key := ""
	env := []string{"PATH=" + os.Getenv("PATH")}
	if r.APIKeyEnv != "" {
		key = os.Getenv(r.APIKeyEnv)
		if key != "" {
			env = append(env, r.APIKeyEnv+"="+key)
		}
	}
	input, err := json.Marshal(execRequest{
		Role: req.Role, Medium: req.Medium, System: req.System, User: req.User, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return provider.Response{}, providerErr("encode request: "+err.Error(), key)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.Command[0], r.Command[1:]...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	stdout := &capWriter{limit: stdoutCap}
	stderr := &capWriter{limit: stderrCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second

	runErr := cmd.Run()
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return provider.Response{}, providerErr(fmt.Sprintf("command timed out after %s", timeout), key)
		}
		return provider.Response{}, providerErr("command cancelled: "+ctx.Err().Error(), key)
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			snippet := redact(stderr.buf.String(), key)
			if len(snippet) > stderrSnippet {
				snippet = snippet[:stderrSnippet]
			}
			return provider.Response{}, providerErr(fmt.Sprintf("command exited with code %d: %s", ee.ExitCode(), strings.TrimSpace(snippet)), key)
		}
		return provider.Response{}, providerErr("run command: "+runErr.Error(), key)
	}
	if stdout.overflow {
		return provider.Response{}, providerErr("command output exceeds 1 MiB", key)
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.buf.Bytes()))
	var out execResponse
	if err := dec.Decode(&out); err != nil {
		return provider.Response{}, providerErr("invalid command output JSON: "+err.Error(), key)
	}
	if _, err := dec.Token(); err != io.EOF {
		return provider.Response{}, providerErr("command output must be exactly one JSON object", key)
	}
	if out.Text == nil {
		return provider.Response{}, providerErr("command output has no text field", key)
	}
	return provider.Response{Text: *out.Text, TokensIn: out.TokensIn, TokensOut: out.TokensOut}, nil
}
