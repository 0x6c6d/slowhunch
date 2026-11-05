#!/usr/bin/env bash
# Verification for T034 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "extract tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestExtract' TestExtractEvidenceDrop TestExtractBadLocator TestExtractConfidenceCrafts TestExtractCaps TestExtractOwnDefaults TestExtractCode TestExtractCharsCap TestExtractPrompt
CMD
expect_fail "negative: bad evidence, bad locator and crafts confidence are dropped with codes" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	text := "The trees are coming into leaf\nLike something almost being said;"
	st := vault.FormatInbox(text, "poem")
	it := pipeline.Item{Medium: "poem", Title: "Trees", Stored: st, Plain: vault.InboxPlain(st, "poem")}
	c := 0.5
	raw := []pipeline.Atom{
		{Claim: "ok", Evidence: "The trees are coming into leaf", Stance: "crafts", Locator: "line 1"},
		{Claim: "para", Evidence: "Trees are leafing", Stance: "crafts", Locator: "line 1"},
		{Claim: "range", Evidence: "Like something almost being said;", Stance: "reads", Locator: "lines 5-6"},
		{Claim: "conf", Evidence: "The trees", Stance: "crafts", Locator: "line 1", Confidence: &c},
	}
	kept, drops := pipeline.CheckAtoms(raw, it, vault.DefaultConfig().Caps)
	if len(kept) != 1 || kept[0].Claim != "ok" {
		os.Exit(0)
	}
	want := map[int]string{1: "EVIDENCE_NOT_FOUND", 2: "BAD_LOCATOR", 3: "CONFIDENCE_NOT_ASSERTS"}
	for _, d := range drops {
		if want[d.Index] != d.Code {
			os.Exit(0)
		}
		delete(want, d.Index)
	}
	if len(want) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
