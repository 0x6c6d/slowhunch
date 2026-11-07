# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make build`: writes `./bin/slowhunch`.
- `make test`: runs `go test ./...`.
- `make lint`: gofmt must print nothing, then `go vet ./...`.
- `make check`: lint + test + build; must exit 0 before any commit.
- One package's tests: `go test ./internal/vault/ -run TestName`.

## Architecture

Go standard library only. `cmd/slowhunch/main.go` calls `cli.Run` and exits with its return code.

- `internal/cli`: command dispatch, global flags, `--json` output envelopes.
- `internal/errs`: shared error type with stable codes and exit codes.
- `internal/frontmatter`: parser and writer for the strict YAML-like frontmatter subset.
- `internal/vault`: vault config, layout, index, lint, verify and transactions.
- `internal/gitops`: the git wrapper; every git call goes through it (argv, never a shell). Pull is `--rebase --autostash` and aborts on conflict; push is plain `push <remote> <branch>`; behind count is read locally. Never fetch, force or rewrite history.
- `internal/intake`: reads and checks input text (normalization, fencing, secret detection); `.pdf` text comes from `pdf.command` via `provider/exec`'s `RunArgv`. Normalization also strips wikilink brackets `[[x]]` to `x` (not for code).
- `internal/provider`: prompts and the `Provider` interface; subpackages `openai` (chat completions), `exec` (local program, JSON over stdio, plus the exported argv runner `RunArgv`: PATH-only env, no shell) and `fake` (tests).
- `internal/pipeline`: per-item steps between intake and the vault (extract, gate, place, write).
- `internal/synth`: collide pair selection and proposals, `review` queries (read-only).
- `internal/search`: in-memory TF-IDF index with cosine similarity.
- `internal/report`: per-run report JSON written to `.slowhunch/reports/`.
- `internal/policy`: repository-wide policy tests.
- Security-critical single sites: path confinement (`vault` `GuardPath`) and atomic writes (`vault` `WriteFile`); every git call (`gitops`); the only `net/http` (`provider/openai`); the only non-git `os/exec` (`provider/exec` `RunArgv`, also used for `pdf.command`); fencing and secret detection (`intake`).
- Tests use temp vaults and the fake/exec providers only; git tests use a bare repo under a temp dir as `origin`, never a real remote. The spec lives in `.harness/harness.md`.

## Delegation workflow: supervise, don't implement

For non-trivial implementation work, use the supervisor/worker loop (`/loop-coding`, full procedure in `.claude/skills/loop-coding/SKILL.md`) instead of implementing directly:

- Project flow, per version `vN`: spec (`.harness/harness.md` for v1, `.harness/versions/vN/spec.md` for later versions, written with `.harness/prompts/new-version.md`) → `.harness/prompts/generate-plan.md` → `.harness/versions/vN/plan.md` (+ the core `.harness/Plan.md`) → `.harness/prompts/generate-tasks.md` → `.harness/versions/vN/Tasks.md` (small index) + `.harness/versions/vN/tasks/<ID>.md` (brief) + `.harness/verify/vN/<ID>.sh` (verification) → loop runs the index row by row and reads a task file only when it dispatches that task; it never reads the folders of other versions.
- Done = a verification script that exits 0, never "looks right"; workers never touch the spec, prompts, skill or this section (the supervisor checks the diff).
- One fresh worker per task, `model` per task (haiku / sonnet / opus); the supervisor verifies and commits itself and never trusts the worker's report.
- Fail → concrete feedback to the same worker; still failing after 3 rounds → `blocked` and ask the user.
