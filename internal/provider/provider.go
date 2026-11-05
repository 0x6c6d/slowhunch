package provider

import (
	"context"
	"sync"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Request is one completion request.
type Request struct {
	Role, Medium, System, User string
	MaxTokens                  int
}

// Response is one completion reply.
type Response struct {
	Text                string
	TokensIn, TokensOut int
}

// Usage sums token counts over calls.
type Usage struct{ TokensIn, TokensOut int }

// Provider completes a request.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Factory builds a Provider from its configuration.
type Factory func(cfg vault.ProviderConfig) (Provider, error)

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds a factory under name; it panics on a duplicate name.
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, ok := registry[name]; ok {
		panic("provider: duplicate registration of " + name)
	}
	registry[name] = f
}

// New builds the provider named by cfg.Name.
func New(cfg vault.ProviderConfig) (Provider, error) {
	regMu.RLock()
	f, ok := registry[cfg.Name]
	regMu.RUnlock()
	if !ok {
		return nil, errs.Fatal(errs.Unsupported, "unknown provider "+cfg.Name, "use openai-compatible or exec")
	}
	return f(cfg)
}

func validRole(role string) bool {
	for _, r := range Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Effective returns the default provider config with the role's non-empty fields applied.
func Effective(cfg vault.Config, role string) vault.ProviderConfig {
	eff := cfg.Provider
	o, ok := cfg.Roles[role]
	if !ok {
		return eff
	}
	if o.Name != "" {
		eff.Name = o.Name
	}
	if o.BaseURL != "" {
		eff.BaseURL = o.BaseURL
	}
	if o.Model != "" {
		eff.Model = o.Model
	}
	if o.APIKeyEnv != "" {
		eff.APIKeyEnv = o.APIKeyEnv
	}
	if len(o.Command) > 0 {
		eff.Command = append([]string(nil), o.Command...)
	}
	return eff
}

// ForRole builds the provider for role (gate, extract, place or collide).
func ForRole(cfg vault.Config, role string) (Provider, error) {
	if !validRole(role) {
		return nil, errs.Fatal(errs.BadInput, "unknown role "+role, "use gate, extract, place or collide")
	}
	return New(Effective(cfg, role))
}
