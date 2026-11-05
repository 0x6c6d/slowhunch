package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func okBody(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
		"usage":   map[string]any{"prompt_tokens": 7, "completion_tokens": 3},
	})
	return string(b)
}

func newClient(t *testing.T, url, keyEnv string) *Client {
	t.Helper()
	p, err := New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: url, Model: "m1", APIKeyEnv: keyEnv})
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*Client)
	c.Backoff = time.Millisecond
	return c
}

func TestOpenAIRequest(t *testing.T) {
	t.Setenv("OAI_TEST_KEY", "test-key")
	var body map[string]any
	var path, method, auth, ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method, auth, ctype = r.URL.Path, r.Method, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("bad body: %v", err)
		}
		io.WriteString(w, okBody(`{"a":1}`))
	}))
	defer srv.Close()
	c := newClient(t, srv.URL+"/", "OAI_TEST_KEY")
	resp, err := c.Complete(context.Background(), provider.Request{Role: "gate", System: "sys", User: "usr", MaxTokens: 50})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/chat/completions" || method != http.MethodPost || auth != "Bearer test-key" || ctype != "application/json" {
		t.Fatalf("path=%q method=%q auth=%q ctype=%q", path, method, auth, ctype)
	}
	if body["model"] != "m1" || body["temperature"] != float64(0) || body["max_tokens"] != float64(50) {
		t.Fatalf("body: %v", body)
	}
	if _, ok := body["temperature"]; !ok {
		t.Fatal("temperature missing")
	}
	rf, _ := body["response_format"].(map[string]any)
	if rf["type"] != "json_object" {
		t.Fatalf("response_format: %v", body["response_format"])
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages: %v", msgs)
	}
	m0, m1 := msgs[0].(map[string]any), msgs[1].(map[string]any)
	if m0["role"] != "system" || m0["content"] != "sys" || m1["role"] != "user" || m1["content"] != "usr" {
		t.Fatalf("messages: %v", msgs)
	}
	if resp.Text != `{"a":1}` || resp.TokensIn != 7 || resp.TokensOut != 3 {
		t.Fatalf("resp: %+v", resp)
	}
}

func TestOpenAINoKey(t *testing.T) {
	t.Setenv("OAI_TEST_KEY", "")
	var has bool
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, has = r.Header["Authorization"]
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		io.WriteString(w, okBody(`{}`))
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "OAI_TEST_KEY")
	if _, err := c.Complete(context.Background(), provider.Request{System: "s", User: "u"}); err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("Authorization header sent without a key")
	}
	if _, ok := body["max_tokens"]; ok {
		t.Fatal("max_tokens should be omitted when 0")
	}
}

func countingServer(statuses []int, final string) (*httptest.Server, *int32) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&n, 1)) - 1
		if i < len(statuses) {
			w.WriteHeader(statuses[i])
			io.WriteString(w, "err")
			return
		}
		io.WriteString(w, final)
	}))
	return srv, &n
}

func TestOpenAIRetry429(t *testing.T) {
	srv, n := countingServer([]int{429}, okBody(`{}`))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	if _, err := c.Complete(context.Background(), provider.Request{}); err != nil {
		t.Fatal(err)
	}
	if *n != 2 {
		t.Fatalf("requests = %d", *n)
	}
}

func TestOpenAIRetryOnce(t *testing.T) {
	srv, n := countingServer([]int{500, 500, 500}, okBody(`{}`))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	_, err := c.Complete(context.Background(), provider.Request{})
	if errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("err = %v", err)
	}
	if *n != 2 {
		t.Fatalf("requests = %d", *n)
	}
}

func TestOpenAINoRetry400(t *testing.T) {
	srv, n := countingServer([]int{400, 400}, okBody(`{}`))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	_, err := c.Complete(context.Background(), provider.Request{})
	if errs.CodeOf(err) != errs.ProviderError || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v", err)
	}
	if *n != 1 {
		t.Fatalf("requests = %d", *n)
	}
}

func TestOpenAIUsageMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"{}"}}]}`)
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	resp, err := c.Complete(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "{}" || resp.TokensIn != 0 || resp.TokensOut != 0 {
		t.Fatalf("resp: %+v", resp)
	}
}

func TestOpenAINoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	if _, err := c.Complete(context.Background(), provider.Request{}); errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIRedact(t *testing.T) {
	t.Setenv("OAI_TEST_KEY", "test-key-secretvalue")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, "invalid key "+r.Header.Get("Authorization"))
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "OAI_TEST_KEY")
	_, err := c.Complete(context.Background(), provider.Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "test-key-secretvalue") || !strings.Contains(err.Error(), "***") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenAIRepair(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			io.WriteString(w, okBody(`not json`))
			return
		}
		io.WriteString(w, okBody(`{"ok":true}`))
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	var out struct{ OK bool }
	if _, err := provider.CallJSON(context.Background(), c, provider.Request{System: "s", User: "u"}, &out, nil); err != nil {
		t.Fatal(err)
	}
	if !out.OK || n != 2 {
		t.Fatalf("out=%+v requests=%d", out, n)
	}
}

func TestOpenAIBadBaseURL(t *testing.T) {
	for _, u := range []string{"file:///x", "ftp://h", "", "127.0.0.1/v1", "http://"} {
		if _, err := New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: u, Model: "m"}); errs.CodeOf(err) != errs.BadInput {
			t.Errorf("%q: err = %v", u, err)
		}
	}
	if _, err := New(vault.ProviderConfig{Name: "openai-compatible", BaseURL: "http://localhost:1/v1"}); err == nil {
		t.Error("missing model accepted")
	}
}

func TestOpenAITimeoutDefault(t *testing.T) {
	c := newClient(t, "https://localhost/v1", "")
	if c.HTTP.Timeout != 60*time.Second {
		t.Fatalf("timeout = %v", c.HTTP.Timeout)
	}
	if c.HTTP.CheckRedirect == nil {
		t.Fatal("CheckRedirect not set")
	}
}

func TestOpenAINoRedirect(t *testing.T) {
	var hits int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, okBody(`{}`))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/chat/completions", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := newClient(t, srv.URL, "")
	if _, err := c.Complete(context.Background(), provider.Request{}); errs.CodeOf(err) != errs.ProviderError {
		t.Fatalf("err = %v", err)
	}
	if hits != 0 {
		t.Fatal("redirect followed")
	}
}
