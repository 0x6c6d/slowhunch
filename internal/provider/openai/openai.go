// Package openai implements the openai-compatible chat completions provider.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Client calls an openai-compatible /chat/completions endpoint.
type Client struct {
	BaseURL, Model, APIKeyEnv string
	HTTP                      *http.Client  // Timeout 60 s by default
	Backoff                   time.Duration // wait before the single retry; default 1 s
}

func init() { provider.Register("openai-compatible", New) }

// New builds a Client from cfg.
func New(cfg vault.ProviderConfig) (provider.Provider, error) {
	u, err := url.Parse(cfg.BaseURL)
	if cfg.BaseURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errs.Fatal(errs.BadInput, "invalid base_url "+fmt.Sprintf("%q", cfg.BaseURL), "use an absolute http or https URL")
	}
	if cfg.Model == "" {
		return nil, errs.Fatal(errs.BadInput, "model is required for openai-compatible", "set provider.model in the config")
	}
	return &Client{
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		APIKeyEnv: cfg.APIKeyEnv,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects are not followed")
			},
		},
		Backoff: time.Second,
	}, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []message      `json:"messages"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat responseFormat `json:"response_format"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
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

// Complete sends one chat completion request, retrying once on 429, 5xx or a network error.
func (c *Client) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []message{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		Temperature:    0,
		ResponseFormat: responseFormat{Type: "json_object"},
		MaxTokens:      req.MaxTokens,
	})
	if err != nil {
		return provider.Response{}, errs.New(errs.ProviderError, "encode request: "+err.Error(), "")
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			t := time.NewTimer(c.Backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return provider.Response{}, errs.New(errs.ProviderError, "request cancelled: "+ctx.Err().Error(), "")
			case <-t.C:
			}
		}
		resp, retry, err := c.do(ctx, hc, endpoint, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return provider.Response{}, lastErr
}

// do performs one HTTP attempt; retry reports whether a failure is retryable.
func (c *Client) do(ctx context.Context, hc *http.Client, endpoint string, body []byte) (provider.Response, bool, error) {
	key := ""
	if c.APIKeyEnv != "" {
		key = os.Getenv(c.APIKeyEnv)
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return provider.Response{}, false, providerErr("build request: "+err.Error(), key)
	}
	hr.Header.Set("Content-Type", "application/json")
	if key != "" {
		hr.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := hc.Do(hr)
	if err != nil {
		return provider.Response{}, ctx.Err() == nil, providerErr("request failed: "+err.Error(), key)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return provider.Response{}, ctx.Err() == nil, providerErr("read response: "+err.Error(), key)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		snippet := redact(string(data), key)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		retry := res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500
		return provider.Response{}, retry, providerErr(fmt.Sprintf("HTTP %d: %s", res.StatusCode, snippet), key)
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return provider.Response{}, false, providerErr("invalid response JSON: "+err.Error(), key)
	}
	if len(cr.Choices) == 0 {
		return provider.Response{}, false, errs.New(errs.ProviderError, "response has no choices", "")
	}
	out := provider.Response{Text: cr.Choices[0].Message.Content}
	if cr.Usage != nil {
		out.TokensIn = cr.Usage.PromptTokens
		out.TokensOut = cr.Usage.CompletionTokens
	}
	return out, false, nil
}
