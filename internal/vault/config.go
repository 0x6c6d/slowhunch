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

	ByMedium map[string]MediumCaps `json:"by_medium,omitempty"`
}

// MediumCaps limits runs and tokens for one medium (0 = unlimited).
type MediumCaps struct {
	RunsPerDay   int `json:"runs_per_day"`
	TokensPerRun int `json:"tokens_per_run"`
}

// CollideConfig tunes collision search.
type CollideConfig struct {
	MaxCosine       float64 `json:"max_cosine"`
	AestheticWeight float64 `json:"aesthetic_weight"`
	MaxPairs        int     `json:"max_pairs"`
}

// ReviewConfig tunes review.
type ReviewConfig struct {
	StaleDays int `json:"stale_days"`
}

// PDFConfig names the PDF text extraction command.
type PDFConfig struct {
	Command []string `json:"command"`
}

// PlaceConfig tunes placement.
type PlaceConfig struct {
	Near             int     `json:"near"`
	DuplicateCosine  float64 `json:"duplicate_cosine"`
	ContradictCosine float64 `json:"contradict_cosine"`
	EchoLow          float64 `json:"echo_low"`
	EchoHigh         float64 `json:"echo_high"`
	ExcerptChars     int     `json:"excerpt_chars"`
	WildcardShare    float64 `json:"wildcard_share"`
}

// GitConfig controls git commits.
type GitConfig struct {
	Commit      bool   `json:"commit"`
	Pull        bool   `json:"pull"`
	RequirePull bool   `json:"require_pull"`
	Push        bool   `json:"push"`
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
}

// Config is the vault configuration.
type Config struct {
	Provider   ProviderConfig                       `json:"provider"`
	Roles      map[string]ProviderConfig            `json:"roles,omitempty"`
	Media      map[string]map[string]ProviderConfig `json:"media,omitempty"`
	Caps       CapsConfig                           `json:"caps"`
	Collide    CollideConfig                        `json:"collide"`
	Review     ReviewConfig                         `json:"review"`
	PDF        PDFConfig                            `json:"pdf"`
	Place      PlaceConfig                          `json:"place"`
	SendPolicy string                               `json:"send_policy"`
	Git        GitConfig                            `json:"git"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Provider: ProviderConfig{Name: "openai-compatible", BaseURL: "http://localhost:11434/v1", Model: "llama3.1", APIKeyEnv: "SLOWHUNCH_API_KEY"},
		Caps:     CapsConfig{AtomsPerAssertsInput: 5, AtomsPerAestheticInput: 3, CharsPerCall: 12000},
		Place: PlaceConfig{Near: 5, DuplicateCosine: 0.80, ContradictCosine: 0.20,
			EchoLow: 0.20, EchoHigh: 0.80, ExcerptChars: 300, WildcardShare: 0.10},
		Collide:    CollideConfig{MaxCosine: 0.15, AestheticWeight: 2.0, MaxPairs: 6},
		Review:     ReviewConfig{StaleDays: 60},
		PDF:        PDFConfig{Command: []string{"pdftotext", "-layout", "{file}", "-"}},
		SendPolicy: "excerpt_only",
		Git:        GitConfig{Commit: true, Pull: true, Remote: "origin"},
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
	if err := validateConfig(c); err != nil {
		return c, err
	}
	return c, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func validateConfig(c Config) error {
	roles := []string{"gate", "extract", "place", "collide"}
	bad := func(msg, key string) error {
		return errs.Fatal(errs.BadInput, msg, "fix config key "+key)
	}
	for k := range c.Roles {
		if !contains(roles, k) {
			return bad("unknown role in config: "+k, "roles."+k)
		}
	}
	for m, rs := range c.Media {
		if !contains(Mediums, m) {
			return bad("unknown medium in config media: "+m, "media."+m)
		}
		for r := range rs {
			if !contains(roles, r) {
				return bad("unknown role in config media."+m+": "+r, "media."+m+"."+r)
			}
		}
	}
	for m, mc := range c.Caps.ByMedium {
		if !contains(Mediums, m) {
			return bad("unknown medium in config caps.by_medium: "+m, "caps.by_medium."+m)
		}
		if mc.RunsPerDay < 0 {
			return bad("negative runs_per_day for "+m, "caps.by_medium."+m+".runs_per_day")
		}
		if mc.TokensPerRun < 0 {
			return bad("negative tokens_per_run for "+m, "caps.by_medium."+m+".tokens_per_run")
		}
	}
	if c.Collide.MaxCosine < 0 || c.Collide.MaxCosine > 1 {
		return bad("collide.max_cosine must be within [0,1]", "collide.max_cosine")
	}
	if !(c.Collide.AestheticWeight > 0) {
		return bad("collide.aesthetic_weight must be > 0", "collide.aesthetic_weight")
	}
	if c.Collide.MaxPairs < 1 {
		return bad("collide.max_pairs must be >= 1", "collide.max_pairs")
	}
	if c.Review.StaleDays < 1 {
		return bad("review.stale_days must be >= 1", "review.stale_days")
	}
	if c.Place.WildcardShare < 0 || c.Place.WildcardShare > 1 {
		return bad("place.wildcard_share must be within [0,1]", "place.wildcard_share")
	}
	if len(c.PDF.Command) == 0 {
		return bad("pdf.command must not be empty", "pdf.command")
	}
	if c.Git.Remote == "" {
		return bad("git.remote must not be empty", "git.remote")
	}
	return nil
}

// MarshalConfig renders the config as indented JSON with a trailing newline.
func MarshalConfig(c Config) []byte {
	b, _ := json.MarshalIndent(c, "", "  ")
	return append(b, '\n')
}
