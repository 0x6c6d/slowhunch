# Tasks v1: slowhunch

_Generated from .harness/versions/v1/plan.md. This file is the loop's persistent state (status and notes
only): update it in place, never regenerate it during a build. Briefs live in `.harness/versions/v1/tasks/<ID>.md`,
verifications in `.harness/verify/v1/<ID>.sh`. IDs are unique within this version._

## Standing constraints

The supervisor prepends these, verbatim, to every worker brief:

- Repository root: `/home/sa/workspace/code/slowhunch`. Go module `github.com/0x6c6d/slowhunch`, go directive `1.27`. Once `Makefile` exists, `make check` (gofmt clean, `go vet ./...`, `go test ./...`, build) must pass for the whole repo when you finish.
- Protected (never modify, create or delete anything in): the whole `.harness/` folder, `.claude/`, `.yipyip/`, `prompt.md`, `prompt_v2.md`, and the "Delegation workflow" section of `CLAUDE.md` (the rest of `CLAUDE.md` only in the docs task that names it). Touch only the files your task names, plus new test files for your own code; `README.md` only in the README task.
- Do not commit, push, pull, stash or rewrite git history; leave all changes in the working tree.
- Go standard library only: no third-party modules, `go.sum` must not exist, nothing installed globally. Format with `gofmt`.
- No network: tests use only in-process fakes, `net/http/httptest` servers and local helper processes; never a real LLM endpoint or internet host. Tests create vaults and git repos only under `t.TempDir()`; never touch a real vault or this repository's own git state.
- No non-test `.go` file may contain the literal text `--force`, `git push` or `git pull` (a release check greps for them): in help or guide text write the add flag as `-force` (Go's flag package accepts both).
- Never write secrets or API keys into files, logs, reports or test fixtures beyond obvious fakes such as `test-key`.
- Keep exported names and signatures exactly as your brief states; other tasks build on them. Do not delete, skip or weaken existing tests.
- Run your task's verification script `bash .harness/verify/v1/<ID>.sh` before reporting; it must exit 0. Do not edit it.

## Task List

| ID | Phase | Title | Depends-on | Model | Status | Notes |
|----|-------|-------|------------|-------|--------|-------|
| T000 | 0 | Preflight | — | supervisor | done | verify passes |
| T001 | 1 | Build scaffold: go.mod, Makefile, LICENSE, gitignore, main stub | T000 | haiku | done | go.mod, Makefile, LICENSE, .gitignore, cmd/slowhunch/main.go |
| T002 | 1 | errs package and CLI core: dispatch, global flags, JSON envelope, exit codes | T001 | sonnet | done | internal/errs, internal/cli core, main.go |
| T003 | 1 | Vault config defaults and minimal init | T002 | sonnet | done | internal/vault/config.go, internal/cli/cmd_init.go + tests |
| T004 | 1 | skill command (guide at most 60 lines) | T002 | haiku | done | internal/cli/cmd_skill.go + test |
| T005 | 2 | Frontmatter subset parser, canonical writer, rev | T002 | opus | done | internal/frontmatter + tests |
| T006 | 2 | Id generation, slugs and collision suffixes | T001 | opus | done | internal/vault/ids.go + test |
| T007 | 2 | Type schema, field validation, locator grammar, links, inbox format | T005 | sonnet | done | internal/vault schema, locator, links, inbox + tests |
| T008 | 2 | Vault write path: confinement, atomic write, CONFLICT_REV, audit, state, test guard | T005 | opus | done | internal/vault store.go, guard.go + tests |
| T009 | 2 | gitops: init, dirty check, exact-path commit, GIT_MISSING | T002 | opus | done | internal/gitops + tests; known gap: staged-new-then-deleted path fails Commit |
| T010 | 2 | Transactions: conflict check, pending-path resume, one commit per run | T008, T009 | opus | done | internal/vault/txn.go + tests |
| T011 | 2 | In-memory index, backlinks, neighbourhood | T007, T008 | sonnet | done | internal/vault/index.go + tests |
| T012 | 3 | Embedded gate/extract/place prompts with version | T005 | sonnet | done | internal/provider/prompts.go + prompts/*.md + test |
| T013 | 3 | Full init command and shared CLI vault helpers | T003, T010, T012 | sonnet | done | cmd_init.go full, vaultctx.go helpers + tests |
| T014 | 3 | create and get commands | T006, T011, T013 | sonnet | done | docjson.go, cmd_create.go, cmd_get.go + tests; defaults state open, status open |
| T015 | 3 | update command: field patch with rev, topic append | T014 | opus | done | cmd_update.go + tests |
| T016 | 3 | delete command: soft to archive, hard with yes | T011, T013, T014 | opus | done | cmd_delete.go + tests |
| T017 | 3 | list command with filters | T011, T013, T014 | sonnet | done | cmd_list.go + tests |
| T018 | 3 | link, unlink, backlinks, graph commands | T011, T013, T014 | sonnet | done | cmd_link.go, cmd_graph.go + tests |
| T019 | 3 | hunch add, evidence, state, due commands | T006, T011, T013, T014 | sonnet | done | cmd_hunch.go + tests |
| T020 | 3 | source facts command | T007, T011, T013 | haiku | done | cmd_source.go + test |
| T021 | 4 | TF-IDF index, cosine, neighbour selection | T001 | sonnet | done | internal/search tfidf, neighbours + tests; build 2000 docs ~0.2s |
| T022 | 4 | search command: ranking, filters, budget, partners | T021, T011, T013 | sonnet | done | search/rank.go, cmd_search.go + tests; hops 0 = bears_on, filter distance<=2 |
| T023 | 4 | lint checks and command | T011, T013 | sonnet | done | vault/lint.go, cmd_lint.go + tests |
| T024 | 4 | verify checks and command | T023, T014 | sonnet | done | vault/verify.go, cmd_verify.go + tests |
| T025 | 4 | Run report type, key order, save and load | T008 | sonnet | done | internal/report + tests |
| T026 | 4 | stats aggregation and command | T025, T011, T013 | sonnet | done | report/stats.go, cmd_stats.go + tests |
| T027 | 5 | Provider interface, registry, fake, JSON call with repair | T012, T003 | opus | done | provider.go, jsoncall.go, fake/fake.go + tests |
| T028 | 5 | openai-compatible provider | T027 | opus | done | provider/openai + tests; only net/http importer |
| T029 | 5 | exec provider with scrubbed environment | T027 | opus | done | provider/exec + tests; scrubbed env, no shell |
| T030 | 5 | Intake reading, input checks, medium inference | T007 | sonnet | done | internal/intake + tests |
| T031 | 5 | Intake normalization and text hash | T001 | sonnet | done | intake/normalize.go + tests |
| T032 | 5 | Prompt fencing, look-alike neutralization, secret detector | T001 | opus | todo | |
| T033 | 6 | Pipeline item types and gate step | T027, T030, T031, T032, T007 | sonnet | todo | |
| T034 | 6 | Extract step with code-enforced atom rules and caps | T033 | sonnet | todo | |
| T035 | 6 | Place step: neighbours, model verdict, code decision | T033, T021 | opus | todo | |
| T036 | 6 | add orchestration core: intake to commit, idempotent, dry-run | T034, T035, T025, T010, T011, T006 | opus | todo | |
| T037 | 6 | add outcomes: corroboration, contradiction, echo, hunch evidence | T036 | opus | todo | |
| T038 | 6 | add crash resume, dirty-file conflict, exact commit | T036 | opus | todo | |
| T039 | 6 | add CLI command and fake-model exec helper for CLI tests | T036, T028, T029, T013 | sonnet | todo | |
| T040 | 7 | Agent sequence, schema and exit-code coverage tests | T015, T016, T018, T019, T023, T039 | sonnet | todo | |
| T041 | 7 | Performance test on 2,000 atoms | T022, T024 | sonnet | todo | |
| T042 | 7 | Policy tests: no push/pull/force, single network and exec sites, test guard | T008, T039 | haiku | todo | |
| T043 | 7 | README with tested quick start | T004, T014, T015, T016, T017, T018, T019, T020, T022, T024, T026, T037, T038, T039, T040 | sonnet | todo | |
| T044 | 7 | CLAUDE.md build commands and architecture notes | T039 | haiku | todo | |
| T045 | 7 | Definition of Done | T000, T001, T002, T003, T004, T005, T006, T007, T008, T009, T010, T011, T012, T013, T014, T015, T016, T017, T018, T019, T020, T021, T022, T023, T024, T025, T026, T027, T028, T029, T030, T031, T032, T033, T034, T035, T036, T037, T038, T039, T040, T041, T042, T043, T044 | supervisor | todo | |

Status values: `todo` · `done` · `blocked` · `superseded`. No pipe characters inside cells.

## Execution Protocol

Run with the `loop-coding` skill (`/loop-coding v1`). The authoritative procedure
(round limit, commit format, Definition-of-Done repair) is in
`.claude/skills/loop-coding/SKILL.md`; in short, per iteration:
1. Select the first `todo` row whose dependencies are all `done`.
2. Read `.harness/versions/v1/tasks/<ID>.md`; dispatch one fresh worker with the row's `Model` and a brief made of the
   Standing constraints, the task's Brief and the path of its verification script.
3. The supervisor runs `.harness/verify/v1/<ID>.sh` itself (incl. its negative checks); never trust
   the worker's claim.
4. Pass → one commit with the work and this file's update. Fail → feedback; still failing
   after 3 rounds → `blocked`.
5. Nothing selectable → run the Global Definition of Done of `.harness/versions/v1/plan.md`; done when it passes.
