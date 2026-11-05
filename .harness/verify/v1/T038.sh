#!/usr/bin/env bash
# Verification for T038 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "pipeline git tests" <<'CMD'
gotest "./internal/pipeline/..." 'TestConflict|TestCommitExact|TestResume|TestNoCommitFlag' TestConflict TestConflictUntrackedTarget TestCommitExact TestResume TestNoCommitFlag
CMD
expect_ok "plan DoD line: gitops and pipeline conflict/commit tests" <<'CMD'
go test ./internal/gitops/... ./internal/pipeline/... -count=1 -run 'TestConflict|TestCommitExact'
CMD
expect_fail "negative: a run touching a dirty hunch aborts with CONFLICT and writes nothing" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/gitops"
	"github.com/0x6c6d/slowhunch/internal/pipeline"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "cprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".slowhunch/state.json\n.slowhunch/audit.log\n.slowhunch/reports/\n"), 0o644)
	gitops.Init(dir)
	v, _ := vault.Open(dir)
	h := &frontmatter.Doc{}
	for _, kv := range [][2]string{{"id", "h1"}, {"type", "hunch"}, {"created", "2026-11-05"}, {"updated", "2026-11-05"}, {"author", "human"}, {"question", "Why do lines stick"}, {"state", "open"}} {
		h.Set(kv[0], kv[1])
	}
	h.Set("evidence", []map[string]string{})
	if _, err := v.PutDoc("hunches/h1.md", h, ""); err != nil {
		os.Exit(125)
	}
	if _, err := gitops.New(dir).Commit([]string{".gitignore", "hunches/h1.md"}, "seed", "b"); err != nil {
		os.Exit(125)
	}
	f, _ := os.OpenFile(filepath.Join(dir, "hunches", "h1.md"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("hand edit\n")
	f.Close()
	fk := &fake.Fake{Replies: map[string][]string{
		"extract": {`{"atoms":[{"claim":"Leaves as speech","evidence":"The trees are coming into leaf","stance":"crafts","locator":"line 1","tags":[]}]}`},
	}}
	pr := map[string]provider.Prompt{}
	for _, r := range provider.Roles {
		pr[r] = provider.Prompt{Role: r, Version: r + "-1", Text: "sys"}
	}
	d := pipeline.Deps{Vault: v, Config: vault.DefaultConfig(), Providers: map[string]provider.Provider{"gate": fk, "extract": fk, "place": fk}, Prompts: pr, Committer: gitops.New(dir), Now: time.Now, Author: "agent:fake/m"}
	_, err := pipeline.Add(context.Background(), d, pipeline.Options{Arg: "The trees are coming into leaf\nLike something almost being said;", Medium: "poem", Title: "Trees", Author: "Philip Larkin", Hunch: "h1"})
	ents, _ := os.ReadDir(filepath.Join(dir, "notes"))
	inb, _ := os.ReadDir(filepath.Join(dir, "inbox"))
	if err == nil || errs.CodeOf(err) != errs.Conflict || len(ents) != 0 || len(inb) != 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
