#!/usr/bin/env bash
# Verification for T033 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "gate tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestGate' TestGateFactual TestGateAesthetic TestGateOwnerSkips TestGateForce TestGateProviderError TestGateRejects TestGateSecret
CMD
expect_fail "negative: provider error keeps nothing; secrets are never sent" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"errors"
	"os"

	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
)

func main() {
	pr := provider.Prompt{Role: "gate", Version: "gate-1", Text: "sys"}
	bad := &fake.Fake{Handler: func(provider.Request) (provider.Response, error) { return provider.Response{}, errors.New("boom") }}
	it := pipeline.Item{Medium: "article", Plain: "Rates rose.", Stored: []byte("Rates rose.\n"), Delim: "SLOWHUNCH-INTAKE-0123456789abcdef"}
	keep, _, _, err := pipeline.Gate(context.Background(), bad, pr, it)
	if keep || err == nil {
		os.Exit(0)
	}
	spy := &fake.Fake{Replies: map[string][]string{"gate": {`{"keep": true}`}}}
	sec := it
	sec.Plain = "token sk-abcdefghijklmnopqrstu"
	sec.Stored = []byte(sec.Plain + "\n")
	sec.Force = true
	keep, _, _, err = pipeline.Gate(context.Background(), spy, pr, sec)
	if keep || err == nil || len(spy.Calls) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
