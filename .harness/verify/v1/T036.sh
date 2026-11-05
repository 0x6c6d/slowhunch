#!/usr/bin/env bash
# Verification for T036 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "add core tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestAddInputErrors|TestAddBasic|TestAddIdempotent|TestAddDryRun|TestFencing|TestSecret' TestAddInputErrors TestAddBasic TestAddIdempotent TestAddDryRun TestFencing TestSecret
CMD
expect_fail "negative: secret input is refused before any provider call and writes nothing" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "aprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	v, err := vault.Open(dir)
	if err != nil {
		os.Exit(125)
	}
	f := &fake.Fake{Handler: func(provider.Request) (provider.Response, error) {
		return provider.Response{Text: `{"keep": true}`}, nil
	}}
	pr := map[string]provider.Prompt{}
	for _, r := range provider.Roles {
		pr[r] = provider.Prompt{Role: r, Version: r + "-1", Text: "sys"}
	}
	d := pipeline.Deps{Vault: v, Config: vault.DefaultConfig(), Providers: map[string]provider.Provider{"gate": f, "extract": f, "place": f}, Prompts: pr, Now: time.Now, Author: "agent:fake/m"}
	_, err = pipeline.Add(context.Background(), d, pipeline.Options{Arg: "My key is sk-abcdefghijklmnopqrstuvwx and more text here.", Medium: "article", Force: true})
	ents, _ := os.ReadDir(filepath.Join(dir, "inbox"))
	if err == nil || errs.CodeOf(err) != errs.BadInput || len(f.Calls) != 0 || len(ents) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
