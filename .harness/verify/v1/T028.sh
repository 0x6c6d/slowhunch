#!/usr/bin/env bash
# Verification for T028 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "openai provider tests" <<'CMD'
gotest "./internal/provider/openai/..." 'TestOpenAI' TestOpenAIRequest TestOpenAINoKey TestOpenAIRetry429 TestOpenAIRetryOnce TestOpenAINoRetry400 TestOpenAIUsageMissing TestOpenAIRedact TestOpenAIRepair TestOpenAIBadBaseURL TestOpenAITimeoutDefault
CMD
expect_ok "net/http imported only by internal/provider/openai (non-test files)" <<'CMD'
test -z "$(grep -rl --include='*.go' '"net/http"' internal cmd | grep -v '_test\.go$' | grep -v '^internal/provider/openai/')"
CMD
expect_fail "negative: non-http base URLs are refused" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/provider/openai"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	if _, err := openai.New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}); err != nil {
		os.Exit(125)
	}
	for _, u := range []string{"file:///etc/passwd", "ftp://example.org/v1", "", "127.0.0.1/v1"} {
		if _, err := openai.New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: u, Model: "m"}); err == nil {
			os.Exit(0)
		}
	}
	os.Exit(1)
}
GO
CMD
expect_fail "negative: the API key never appears in an error message" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/openai"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("bad key: " + r.Header.Get("Authorization")))
	}))
	defer srv.Close()
	os.Setenv("PROBE_KEY", "sk-probesecret1234567890")
	p, err := openai.New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: srv.URL, Model: "m", APIKeyEnv: "PROBE_KEY"})
	if err != nil {
		os.Exit(125)
	}
	_, err = p.Complete(context.Background(), provider.Request{Role: "gate", System: "s", User: "u"})
	if err == nil {
		os.Exit(125)
	}
	if strings.Contains(err.Error(), "probesecret") {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
