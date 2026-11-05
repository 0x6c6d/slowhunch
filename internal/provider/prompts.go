// Package provider holds the embedded default prompts and their loader.
package provider

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

//go:embed prompts/*.md
var promptFS embed.FS

// Prompt is a system prompt with its version.
type Prompt struct{ Role, Version, Text string }

// Roles lists the prompt roles.
var Roles = []string{"gate", "extract", "place"}

// DefaultPrompts returns role -> embedded file bytes.
func DefaultPrompts() map[string][]byte {
	m := map[string][]byte{}
	for _, r := range Roles {
		b, err := promptFS.ReadFile("prompts/" + r + ".md")
		if err == nil {
			m[r] = b
		}
	}
	return m
}

// ParsePrompt parses a prompt file for the given role.
func ParsePrompt(role string, b []byte) (Prompt, error) {
	d, err := frontmatter.Parse(b)
	if err != nil {
		return Prompt{}, errs.New(errs.BadInput, "prompt "+role+": "+err.Error(), "fix the frontmatter")
	}
	if got := d.GetString("role"); got != role {
		return Prompt{}, errs.New(errs.BadInput, "prompt "+role+": role is "+got, "set role: "+role)
	}
	v := d.GetString("version")
	if v == "" {
		return Prompt{}, errs.New(errs.BadInput, "prompt "+role+": version is empty", "add a version field")
	}
	return Prompt{Role: role, Version: v, Text: d.Body}, nil
}

// LoadPrompts reads <root>/.slowhunch/prompts/<role>.md, falling back to defaults.
func LoadPrompts(vaultRoot string) (map[string]Prompt, error) {
	defs := DefaultPrompts()
	out := map[string]Prompt{}
	for _, r := range Roles {
		b, err := os.ReadFile(filepath.Join(vaultRoot, ".slowhunch", "prompts", r+".md"))
		if errors.Is(err, fs.ErrNotExist) {
			b, err = defs[r], nil
		}
		if err != nil {
			return nil, errs.New(errs.BadInput, "prompt "+r+": "+err.Error(), "check the prompt file")
		}
		p, err := ParsePrompt(r, b)
		if err != nil {
			return nil, err
		}
		out[r] = p
	}
	return out, nil
}

// Versions returns role -> version.
func Versions(p map[string]Prompt) map[string]string {
	m := map[string]string{}
	for r, x := range p {
		m[r] = x.Version
	}
	return m
}
