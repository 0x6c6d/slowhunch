package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

func writeCfg(t *testing.T, s string) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigPath(root), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestConfigDefaults(t *testing.T) {
	root := writeCfg(t, string(MarshalConfig(DefaultConfig())))
	c, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	d := DefaultConfig()
	if c.Provider.Name != d.Provider.Name || c.Provider.BaseURL != "http://localhost:11434/v1" || c.Provider.Model != "llama3.1" || c.Provider.APIKeyEnv != "SLOWHUNCH_API_KEY" ||
		c.Caps != (CapsConfig{5, 3, 12000}) || c.Place != (PlaceConfig{5, 0.8, 0.2, 0.2, 0.8, 300}) ||
		c.SendPolicy != "excerpt_only" || !c.Git.Commit {
		t.Fatalf("bad defaults: %+v", c)
	}
}

func TestConfigPartialFile(t *testing.T) {
	c, err := LoadConfig(writeCfg(t, `{"git":{"commit":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Git.Commit || c.Caps.CharsPerCall != 12000 || c.Provider.Model != "llama3.1" {
		t.Fatalf("%+v", c)
	}
}

func TestConfigInvalid(t *testing.T) {
	_, err := LoadConfig(writeCfg(t, `{bad`))
	if errs.CodeOf(err) != errs.BadInput || errs.ExitCode(err) != 1 {
		t.Fatalf("got %v", err)
	}
	if _, err := LoadConfig(t.TempDir()); errs.ExitCode(err) != 1 {
		t.Fatalf("missing file: %v", err)
	}
	_, err = LoadConfig(writeCfg(t, `{"send_policy":"full"}`))
	if errs.CodeOf(err) != errs.Unsupported || errs.ExitCode(err) != 1 {
		t.Fatalf("got %v", err)
	}
}
