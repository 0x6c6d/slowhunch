#!/usr/bin/env bash
# Verification for T037 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "add outcome tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestAddLarkin|TestAddPlaceThresholds|TestAddContradiction|TestAddGateError|TestAddForce|TestAddEvidenceDrops' TestAddLarkin TestAddPlaceThresholds TestAddContradiction TestAddGateError TestAddForce TestAddEvidenceDrops
CMD
expect_ok "all pipeline TestAdd tests pass together" <<'CMD'
go test ./internal/pipeline/... -count=1 -run 'TestAdd'
CMD
expect_fail "negative: a gate provider error writes nothing" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "oprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	v, _ := vault.Open(dir)
	f := &fake.Fake{Handler: func(provider.Request) (provider.Response, error) { return provider.Response{}, errors.New("down") }}
	pr := map[string]provider.Prompt{}
	for _, r := range provider.Roles {
		pr[r] = provider.Prompt{Role: r, Version: r + "-1", Text: "sys"}
	}
	d := pipeline.Deps{Vault: v, Config: vault.DefaultConfig(), Providers: map[string]provider.Provider{"gate": f, "extract": f, "place": f}, Prompts: pr, Now: time.Now, Author: "agent:fake/m"}
	rep, err := pipeline.Add(context.Background(), d, pipeline.Options{Arg: "Interest rates rose by two points in 2024 according to the central bank report.", Medium: "article", Title: "Rates"})
	if err != nil || rep == nil {
		os.Exit(125)
	}
	for _, sub := range []string{"inbox", "sources", "notes"} {
		ents, _ := os.ReadDir(filepath.Join(dir, sub))
		if len(ents) != 0 {
			os.Exit(0)
		}
	}
	if len(rep.Errors) == 0 || len(rep.Atoms) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
