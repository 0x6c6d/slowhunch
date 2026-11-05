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
		c.Caps.AtomsPerAssertsInput != 5 || c.Caps.AtomsPerAestheticInput != 3 || c.Caps.CharsPerCall != 12000 || c.Place != (PlaceConfig{5, 0.8, 0.2, 0.2, 0.8, 300, 0.10}) ||
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

func TestConfigV2(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		c := DefaultConfig()
		if c.Collide != (CollideConfig{0.15, 2.0, 6}) || c.Review.StaleDays != 60 || c.Place.WildcardShare != 0.10 ||
			len(c.PDF.Command) != 4 || c.PDF.Command[0] != "pdftotext" || c.PDF.Command[2] != "{file}" ||
			len(c.Media) != 0 || len(c.Caps.ByMedium) != 0 ||
			c.Git != (GitConfig{Commit: true, Pull: true, Remote: "origin"}) {
			t.Fatalf("%+v", c)
		}
		l, err := LoadConfig(writeCfg(t, string(MarshalConfig(c))))
		if err != nil || l.Collide != c.Collide || l.Git != c.Git {
			t.Fatalf("round trip: %v %+v", err, l)
		}
	})
	t.Run("partial", func(t *testing.T) {
		c, err := LoadConfig(writeCfg(t, `{"collide":{"max_pairs":9},"review":{"stale_days":7},"place":{"wildcard_share":0.5},
"pdf":{"command":["x","{file}"]},"git":{"push":true},
"media":{"paper":{"gate":{"name":"fake"}}},"caps":{"by_medium":{"film":{"runs_per_day":3}}}}`))
		if err != nil {
			t.Fatal(err)
		}
		if c.Collide != (CollideConfig{0.15, 2.0, 9}) || c.Review.StaleDays != 7 || c.Place.WildcardShare != 0.5 || c.Place.Near != 5 ||
			len(c.PDF.Command) != 2 || !c.Git.Push || !c.Git.Pull || !c.Git.Commit || c.Git.RequirePull || c.Git.Remote != "origin" ||
			c.Media["paper"]["gate"].Name != "fake" || c.Caps.ByMedium["film"] != (MediumCaps{3, 0}) || c.Caps.CharsPerCall != 12000 {
			t.Fatalf("%+v", c)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		for _, s := range []string{
			`{"roles":{"gatee":{"model":"x"}}}`,
			`{"media":{"podcast":{"gate":{"name":"x"}}}}`,
			`{"media":{"paper":{"gatee":{"name":"x"}}}}`,
			`{"caps":{"by_medium":{"podcast":{"runs_per_day":1}}}}`,
			`{"caps":{"by_medium":{"paper":{"runs_per_day":-1}}}}`,
			`{"caps":{"by_medium":{"paper":{"tokens_per_run":-1}}}}`,
			`{"collide":{"max_cosine":1.5}}`, `{"collide":{"max_cosine":-0.1}}`,
			`{"collide":{"aesthetic_weight":0}}`, `{"collide":{"max_pairs":0}}`,
			`{"review":{"stale_days":0}}`,
			`{"place":{"wildcard_share":2}}`, `{"place":{"wildcard_share":-1}}`,
			`{"pdf":{"command":[]}}`, `{"git":{"remote":""}}`,
		} {
			_, err := LoadConfig(writeCfg(t, s))
			if errs.CodeOf(err) != errs.BadInput {
				t.Errorf("%s: got %v", s, err)
			}
		}
	})
	t.Run("state", func(t *testing.T) {
		v := &Vault{Root: t.TempDir()}
		if err := os.MkdirAll(filepath.Join(v.Root, ".slowhunch"), 0o755); err != nil {
			t.Fatal(err)
		}
		in := State{RunsByDay: map[string]map[string]int{"2026-01-02": {"paper": 2}}}
		if err := v.SaveState(in); err != nil {
			t.Fatal(err)
		}
		out, err := v.LoadState()
		if err != nil || out.RunsByDay["2026-01-02"]["paper"] != 2 {
			t.Fatalf("%v %+v", err, out)
		}
	})
}
