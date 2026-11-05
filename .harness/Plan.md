# Plan: slowhunch

_Core plan, valid for all versions; edited in place by the planner of each version (git is the
history). Phases, Definition of Done and requirement coverage live in `.harness/versions/vN/plan.md`._

## Scope
**Product scope so far:** v1: Go CLI `slowhunch` (stdlib only) + plain-Markdown vault in git: `init`, `add`
pipeline (intake → gate → extract → place → write → report), CRUD/links/hunches, `search` (TF-IDF), `lint`,
`verify`, `stats`, `skill`, providers `openai-compatible` + `exec`, one local git commit per run, README.
v2: `collide` (proposals), `review`, wildcard lane, `keep` (topic append), batch `add --from`, PDF text via
`pdf.command`, per-role/per-medium providers, per-medium caps, git pull-before-write/optional push/`sync`.
**Explicitly deferred (product level):** `docs/`, `CONTRIBUTING.md`, `examples/`, MCP server,
embeddings/vector DB, OCR/vision/audio, daemon/scheduler, multi-user conflict resolution, vault import,
`send_policy` other than `excerpt_only`, `prompts diff`, `git fetch`. `anthropic` provider: dropped for good.
**Where the code lives / protected paths (no worker may touch):** code at repo root
`/home/sa/workspace/code/slowhunch` (`go.mod`, `cmd/slowhunch/`, `internal/`, `Makefile`, `LICENSE`,
`.gitignore`, `README.md`). Protected: the whole `.harness/` folder, `.claude/`, the "Delegation workflow"
section of `CLAUDE.md` (only that section), `.yipyip/`. `README.md` is written
only by the README task.

## Architecture
Module `github.com/0x6c6d/slowhunch`, go directive `1.27`, no third-party imports (`go.sum` stays absent).
- **cmd/slowhunch**: `main.go` only; calls `cli.Run(os.Args[1:], stdin, stdout, stderr) int` and exits with
  its code. (since v1)
- **internal/cli**: flag parsing per subcommand (`flag`), global flags `--json --vault --dry-run
  --no-commit`, vault lookup (`--vault` → `SLOWHUNCH_VAULT` → nearest parent with `.slowhunch/`), JSON
  envelope `{"schema":1,...}`, error `{code,message,hint}`, exit codes 0/1/2/3/4; commands `init add
  create get update delete list link unlink backlinks graph hunch{add,evidence,state,due} source facts
  search lint verify stats skill`. Talks to: vault, pipeline, search, gitops, report. (since v1) v2: `collide
  review keep sync`, `add --from`, flags `--push --no-push --no-pull --sync`; one run wrapper: resume →
  dirty check → pull → index rebuild → command → commit → push; codes `CAP_REACHED PULL_FAILED PUSH_FAILED BEHIND`.
- **internal/frontmatter**: strict own subset parser/writer (scalars, string lists, list of maps for hunch
  `evidence`), canonical byte form, `rev` = first 6 hex of sha256 of canonical bytes minus the `rev` line.
  Parses untrusted files: rejects anything outside the subset. (since v1)
- **internal/vault**: types `note source hunch proposal topic`, field validation, id generation (R4) and
  collision suffixes, locator grammar (R6), `[[id]]` link parse + computed backlinks, the single
  write path (temp+rename, symlink-resolved confinement, `CONFLICT_REV`, audit log
  `.slowhunch/audit.log`, `state.json`), soft/hard delete, test guard (R24), in-memory index rebuild.
  Talks to: frontmatter. (since v1) v2: config `collide review place.wildcard_share pdf media caps.by_medium
  git.{pull,require_pull,push,remote,branch}`, `state.json` `runs_by_day`, topic append for `keep`.
- **internal/intake**: input reading (arg/stdin/file, `.txt`/`.md` only), size/empty checks, medium
  inference, normalization, line numbering, `text_hash`, immutable `inbox/<source-id>.txt`, prompt fencing
  with random per-run delimiter + look-alike neutralization, secret-pattern detector. (since v1)
- **internal/provider**: `Provider` interface `Complete(ctx, Request) (Response, error)` + registry;
  `openai-compatible` (`net/http`, the only outbound network code), `exec` (single `os/exec` call site, no
  shell, scrubbed env); JSON request/validate/repair-once helper; embedded prompts (`embed`) with
  `version`; test-only fake provider in `internal/provider/fake`. (since v1) v2: role `collide` + `collide.md`;
  resolution `media.<medium>.<role>` > `roles.<role>` > `provider` field by field; `tokens_per_run` meter;
  `provider/exec` exports its scrubbed-env argv runner, also used for `pdf.command` (intake `.pdf`).
- **internal/synth**: `collide` pair selection (cosine <= `max_cosine`, aesthetic weight, reserved `asserts` x
  `reads` cross-medium quota, deterministic) + proposal build; `review` queries (read-only). Talks to: search,
  vault, provider. (since v2)
- **internal/search**: TF-IDF index over atoms (stdlib), cosine similarity, ranked search (score,
  recency, hops), filters, `--budget` token estimate (chars/4), neighbour selection for place. (since v1)
- **internal/pipeline**: `add` orchestration: gate → extract (code-enforced evidence/locator/confidence
  rules, caps) → place (model verdict, code decides with `config.place` thresholds) → write atoms,
  source, corroborations, contradictions, hunch evidence → report; idempotency and crash resume. Talks to:
  intake, provider, search, vault, gitops, report. (since v1) v2: batch runner (`--from`, one run id),
  wildcard lane, per-medium caps.
- **internal/report**: run report with ordered keys (`contradictions hunches_moved atoms corroborations
  echoes wildcards collisions counts errors cost git`), written to `.slowhunch/reports/<run-id>.json`;
  `stats` aggregation. (since v1)
- **internal/gitops**: `git` via `os/exec` argv only: init, dirty-file check (`CONFLICT`), stage exactly
  listed paths, commit with message `slowhunch: <command> <ids> (<counts>)` + run id/prompt versions body,
  `GIT_MISSING` detection. (since v1) v2: `pull --rebase --autostash` (+ `rebase --abort` on conflict), plain
  `push <remote> <branch>`, `rev-list --count` for `pulled`/`BEHIND`; never fetch/force/reset/history rewrite.
- Data flow `add`: agent text → intake → provider(gate) → provider(extract) → search(neighbours) →
  provider(place) → vault writes → gitops commit → report on stdout. External integration: only the
  configured provider `base_url`, the local `git` binary (v2: also talking to the vault's own remote) and
  `pdf.command`.

## Commands
- install: `go mod download`
- test: `make test` (`go test ./...`)
- lint: `make lint` (`test -z "$(gofmt -l .)"` and `go vet ./...`)
- build: `make build` (writes `./bin/slowhunch`)
- check (runs all of the above, exit 0 = healthy): `make check`

## Model-Routing Policy
Tier per task via the `Agent` tool's `model`. Quality first, then cost: never pick a lower tier if a wrong
result costs more to redo. Precedence: a security or Consequential rule beats a cost example.
- **haiku**: clear boundary, small diff, mechanically verifiable. Examples: `Makefile`/`LICENSE`/
  `.gitignore`; `skill` guide text command; adding a filter flag to `list` following an existing one.
- **sonnet** (default): multi-file features, moderate judgment, new patterns. Examples: CRUD/link/hunch
  commands on top of the vault write path; TF-IDF `search` and `stats`; `lint`/`verify` checks; report
  assembly; README task (fixed tier, never raised or lowered).
- **opus**: architecture, security-sensitive code, concurrency, cross-cutting refactors, Consequential
  assumptions. Examples: `frontmatter` parser (untrusted input); `vault` write path (confinement, atomic
  write, rev/CONFLICT_REV, test guard); `gitops` and `exec` provider (subprocess construction, env scrub);
  `openai-compatible` (secrets, only network egress); `intake` fencing + secret detector; `place` decision
  code and the `add` orchestration (data loss, idempotency).
- **Shared-code rule:** security-critical logic lives in one opus-built place each: path confinement and
  atomic write in `vault`, every `git` invocation in `gitops`, the only `os/exec` for providers in
  `provider/exec`, the only `net/http` client in `provider/openai`, fencing/secret check in `intake`.
  Modules that only call these (cli commands, search, report) may be sonnet; the supervisor then runs the
  negative checks (escape paths, dirty-file conflict, secret not sent, no extra network host).

## Risks & Open Questions
- **Own frontmatter subset (schema choice, expensive to change):** files outside the subset are rejected
  by `verify`/`get` with `BAD_INPUT`, never rewritten; hand-edited vaults may fail until fixed.
- **Path confinement / prompt injection:** model output is never a path or command; slugs sanitized to
  `[a-z0-9-]`; every write goes through the single vault write path with symlink-resolved check.
- **Secrets:** key read only from env var named in `api_key_env`; redacted (`***`) in errors; secret-like
  patterns (`sk-[A-Za-z0-9]{16,}`, `AKIA[0-9A-Z]{16}`, `-----BEGIN .*PRIVATE KEY-----`, `ghp_[A-Za-z0-9]{36}`,
  `xox[bp]-`) block sending: the item fails with `BAD_INPUT` and nothing is sent or written.
- **Data loss:** `evidence` and `inbox/` files are never rewritten; `delete` is soft by default, `--hard`
  needs `--yes`; dirty foreign files abort with `CONFLICT` before any write.
- **Git side effects:** commits only listed paths in the vault repo; never force, `reset --hard` or history
  rewrite (policy-tested, v2 replaces the v1 push/pull word ban); push is opt-in (`git.push` false).
  Crash resume: uncommitted files recorded in `.slowhunch/state.json` `pending` are committed by the next
  run of any vault-changing command.
- **Network egress:** only `provider.base_url`; tests use `httptest` and the fake/exec provider only.
- **Legal/copyright:** 12,000-char cap per input, one work per `add`; README tells owners to keep remotes
  private.
- **Performance:** in-memory TF-IDF must keep `verify`/`search` < 5 s on 2,000 atoms; if not, optimize
  index build, never add an on-disk cache in v1.
- **Provider API drift:** `openai-compatible` relies on `response_format json_object` and `usage`; a missing
  `usage` records 0 tokens rather than failing.
- **Pull/rebase (v2, live remote):** pull only with remote + upstream, after the dirty check, before provider
  calls; a rebase conflict is aborted, HEAD/status asserted unchanged, `CONFLICT` exit 4; unreachable remote
  = `PULL_FAILED` warning (exit 1 only with `git.require_pull`); tests use a `t.TempDir()` bare remote only.
- **Push (v2):** plain `git push <remote> <branch>`; failure = `PUSH_FAILED` warning, commit kept, retried next run.
- **PDF subprocess (v2):** `pdf.command` runs through the `provider/exec` runner (argv, `PATH`-only env, 60 s);
  `{file}` substituted as a whole argv element only; extracted text passes all intake checks.
