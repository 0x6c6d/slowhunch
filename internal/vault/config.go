// Package vault holds vault configuration and layout.
package vault

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

// ProviderConfig describes one model provider.
type ProviderConfig struct {
	Name      string   `json:"name"`
	BaseURL   string   `json:"base_url,omitempty"`
	Model     string   `json:"model,omitempty"`
	APIKeyEnv string   `json:"api_key_env,omitempty"`
	Command   []string `json:"command,omitempty"`
}

// CapsConfig limits input sizes.
type CapsConfig struct {
	AtomsPerAssertsInput   int `json:"atoms_per_asserts_input"`
	AtomsPerAestheticInput int `json:"atoms_per_aesthetic_input"`
	CharsPerCall           int `json:"chars_per_call"`
}

// PlaceConfig tunes placement.
type PlaceConfig struct {
	Near             int     `json:"near"`
	DuplicateCosine  float64 `json:"duplicate_cosine"`
	ContradictCosine float64 `json:"contradict_cosine"`
	EchoLow          float64 `json:"echo_low"`
	EchoHigh         float64 `json:"echo_high"`
	ExcerptChars     int     `json:"excerpt_chars"`
}

// GitConfig controls git commits.
type GitConfig struct {
	Commit bool `json:"commit"`
}

// Config is the vault configuration.
type Config struct {
	Provider   ProviderConfig            `json:"provider"`
	Roles      map[string]ProviderConfig `json:"roles,omitempty"`
	Caps       CapsConfig                `json:"caps"`
	Place      PlaceConfig               `json:"place"`
	SendPolicy string                    `json:"send_policy"`
	Git        GitConfig                 `json:"git"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Provider: ProviderConfig{Name: "openai-compatible", BaseURL: "http://localhost:11434/v1", Model: "llama3.1", APIKeyEnv: "SLOWHUNCH_API_KEY"},
		Caps:     CapsConfig{AtomsPerAssertsInput: 5, AtomsPerAestheticInput: 3, CharsPerCall: 12000},
		Place: PlaceConfig{Near: 5, DuplicateCosine: 0.80, ContradictCosine: 0.20,
			EchoLow: 0.20, EchoHigh: 0.80, ExcerptChars: 300},
		SendPolicy: "excerpt_only",
		Git:        GitConfig{Commit: true},
	}
}

// ConfigPath returns <root>/.slowhunch/config.json.
func ConfigPath(root string) string {
	return filepath.Join(root, ".slowhunch", "config.json")
}

// LoadConfig reads the config file over the defaults.
func LoadConfig(root string) (Config, error) {
	c := DefaultConfig()
	p := ConfigPath(root)
	data, err := os.ReadFile(p)
	if err != nil {
		return c, errs.Fatal(errs.BadInput, "cannot read config: "+err.Error(), "run slowhunch init")
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return DefaultConfig(), errs.Fatal(errs.BadInput, "invalid config "+p+": "+err.Error(), "fix the JSON")
	}
	if c.SendPolicy != "excerpt_only" {
		return c, errs.Fatal(errs.Unsupported, "unsupported send_policy: "+c.SendPolicy, "only excerpt_only is supported")
	}
	return c, nil
}

// MarshalConfig renders the config as indented JSON with a trailing newline.
func MarshalConfig(c Config) []byte {
	b, _ := json.MarshalIndent(c, "", "  ")
	return append(b, '\n')
}
