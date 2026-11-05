package provider_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

type stub struct{ cfg vault.ProviderConfig }

func (s *stub) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	return provider.Response{Text: "{}"}, nil
}

func init() {
	provider.Register("test-stub", func(cfg vault.ProviderConfig) (provider.Provider, error) {
		return &stub{cfg: cfg}, nil
	})
}

func TestRegistry(t *testing.T) {
	p, err := provider.New(vault.ProviderConfig{Name: "test-stub", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := p.(*stub); !ok || s.cfg.Model != "m" {
		t.Fatalf("got %#v", p)
	}
	_, err = provider.New(vault.ProviderConfig{Name: "nope"})
	if errs.CodeOf(err) != errs.Unsupported || errs.ExitCode(err) != 1 {
		t.Fatalf("unknown provider: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register did not panic")
		}
	}()
	provider.Register("test-stub", nil)
}

func TestForRole(t *testing.T) {
	cfg := vault.Config{
		Provider: vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "small", APIKeyEnv: "K"},
		Roles:    map[string]vault.ProviderConfig{"place": {Model: "big"}},
	}
	eff := provider.Effective(cfg, "place", "")
	if eff.Model != "big" || eff.BaseURL != "http://base" || eff.Name != "test-stub" || eff.APIKeyEnv != "K" {
		t.Fatalf("effective: %+v", eff)
	}
	if provider.Effective(cfg, "gate", "").Model != "small" {
		t.Fatal("gate should use default")
	}
	p, err := provider.ForRole(cfg, "place", "")
	if err != nil || p.(*stub).cfg.Model != "big" {
		t.Fatalf("ForRole: %v", err)
	}
	if _, err := provider.ForRole(cfg, "bogus", ""); err == nil {
		t.Fatal("bogus role accepted")
	}
}

type out struct {
	Keep bool `json:"keep"`
}

func TestCallJSONOK(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"gate": {"  ```json\n{\"keep\": true}\n```  "}}}
	var o out
	u, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "hello world!"}, &o, nil)
	if err != nil || !o.Keep || len(f.Calls) != 1 || u.TokensIn != 3 {
		t.Fatalf("err=%v o=%+v calls=%d u=%+v", err, o, len(f.Calls), u)
	}
	f = &fake.Fake{Replies: map[string][]string{"gate": {"```\n{\"keep\": true}\n```"}}}
	if _, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate"}, &o, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCallJSONRepair(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"gate": {"sure! here it is", "{\"keep\": true}"}}}
	var o out
	u, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "orig user"}, &o, nil)
	if err != nil || !o.Keep {
		t.Fatalf("err=%v o=%+v", err, o)
	}
	calls := f.CallsFor("gate")
	if len(calls) != 2 {
		t.Fatalf("calls=%d", len(calls))
	}
	u2 := calls[1].User
	if !strings.HasPrefix(u2, "orig user\n\nYour previous reply was invalid: ") ||
		!strings.HasSuffix(u2, ". Reply with exactly one JSON object and nothing else.") {
		t.Fatalf("repair user: %q", u2)
	}
	wantIn := len("orig user")/4 + len(u2)/4
	wantOut := len("sure! here it is")/4 + len("{\"keep\": true}")/4
	if u.TokensIn != wantIn || u.TokensOut != wantOut {
		t.Fatalf("usage %+v want %d/%d", u, wantIn, wantOut)
	}
}

func TestCallJSONFailsAfterRepair(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"gate": {"nope", "{\"keep\": true} {}", "{\"keep\": true}"}}}
	var o out
	_, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "u"}, &o, nil)
	if errs.CodeOf(err) != errs.ProviderError || !strings.Contains(err.Error(), "invalid model output after one repair") {
		t.Fatalf("err=%v", err)
	}
	if len(f.Calls) != 2 {
		t.Fatalf("calls=%d", len(f.Calls))
	}
}

func TestCallJSONValidate(t *testing.T) {
	f := &fake.Fake{Replies: map[string][]string{"extract": {"{\"keep\": false}", "{\"keep\": true}"}}}
	var o out
	validate := func() error {
		if !o.Keep {
			return errors.New("keep must be true")
		}
		return nil
	}
	_, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "extract", User: "u"}, &o, validate)
	if err != nil || !o.Keep || len(f.Calls) != 2 {
		t.Fatalf("err=%v calls=%d", err, len(f.Calls))
	}
	if !strings.Contains(f.Calls[1].User, "keep must be true") {
		t.Fatalf("repair message lacks validation error: %q", f.Calls[1].User)
	}
}

func TestCallJSONProviderError(t *testing.T) {
	f := &fake.Fake{Handler: func(req provider.Request) (provider.Response, error) {
		return provider.Response{}, errors.New("connection refused")
	}}
	var o out
	_, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "u"}, &o, nil)
	if errs.CodeOf(err) != errs.ProviderError || len(f.Calls) != 1 {
		t.Fatalf("err=%v calls=%d", err, len(f.Calls))
	}
}

func TestResolveV2(t *testing.T) {
	cfg := vault.Config{
		Provider: vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "small", APIKeyEnv: "K0", Command: []string{"a", "b"}},
		Roles: map[string]vault.ProviderConfig{
			"extract": {Model: "role-m", APIKeyEnv: "K1"},
			"place":   {Command: []string{"x"}},
		},
		Media: map[string]map[string]vault.ProviderConfig{
			"poem": {"extract": {Model: "poem-m"}},
		},
	}
	base := vault.Config{Provider: cfg.Provider}
	tests := []struct {
		name         string
		cfg          vault.Config
		role, medium string
		want         vault.ProviderConfig
	}{
		{"provider only", base, "gate", "poem", cfg.Provider},
		{"role model only", cfg, "extract", "", vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "role-m", APIKeyEnv: "K1", Command: []string{"a", "b"}}},
		{"media beats role", cfg, "extract", "poem", vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "poem-m", APIKeyEnv: "K1", Command: []string{"a", "b"}}},
		{"other medium no effect", cfg, "extract", "article", vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "role-m", APIKeyEnv: "K1", Command: []string{"a", "b"}}},
		{"command replaced whole", cfg, "place", "poem", vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "small", APIKeyEnv: "K0", Command: []string{"x"}}},
		{"empty medium", cfg, "extract", "", vault.ProviderConfig{Name: "test-stub", BaseURL: "http://base", Model: "role-m", APIKeyEnv: "K1", Command: []string{"a", "b"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := provider.Effective(tc.cfg, tc.role, tc.medium)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
	if _, err := provider.ForRole(cfg, "bogus", "poem"); err == nil {
		t.Fatal("unknown role accepted")
	}
}
