# Tasks v2: slowhunch

_Generated from .harness/versions/v2/plan.md. This file is the loop's persistent state (status and notes
only): update it in place, never regenerate it during a build. Briefs live in `.harness/versions/v2/tasks/<ID>.md`,
verifications in `.harness/verify/v2/<ID>.sh`. IDs are unique within this version._

## Standing constraints

The supervisor prepends these, verbatim, to every worker brief:

- `make check` (gofmt prints nothing, `go vet ./...`, `go test ./...`, `go build`) must pass for the whole repo.
- Protected (never modify): the whole `.harness/` folder, `.claude/`, `.yipyip/`, and the "Delegation workflow"
  section of `CLAUDE.md`. `README.md` is not protected, but only the README task edits it; stay out of every
  file your brief does not name, except tests that your change must adapt (say which in your report).
- Do not commit, push, pull, stash or rewrite history in this repository; the supervisor commits.
- Go standard library only (`go.sum` stays absent); module `github.com/0x6c6d/slowhunch`, go 1.27.
- No network: tests use temp vaults (`t.TempDir()`), the fake provider (`internal/provider/fake`) or the exec
  provider with the test binary as helper, `httptest` on loopback only, and for git remotes only a bare repo
  under `t.TempDir()` used by file path; never this repository's own remote. Tests that run git set
  `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` and author/committer env vars.
- Security-critical single sites stay single: path confinement and atomic writes in `internal/vault`, every
  `git` invocation in `internal/gitops` (argv, never a shell), the only `net/http` in `internal/provider/openai`,
  the only non-git `os/exec` in `internal/provider/exec`, fencing and secret detection in `internal/intake`.
  Model output is never a path or a command.
- Never weaken or delete an existing test unless your brief says so; tests for your code are part of your task.
- The verification script named in your brief is supervisor-owned: do not edit it; make it exit 0.

## Task List

| ID | Phase | Title | Depends-on | Model | Status | Notes |
|----|-------|-------|------------|-------|--------|-------|
| T000 | 0 | Preflight | — | supervisor | done | preflight ok |
| T001 | 1 | Config v2 keys, defaults and validation | T000 | sonnet | done | config.go, store.go, config_test.go; v2 config keys + validation |
| T002 | 1 | Error codes v2 | T000 | haiku | done | errs.go, errs_test.go; v2 codes + exits |
| T003 | 1 | Git policy tests: force and rewrite ban | T000 | opus | done | policy_test.go; TestNoForceGit + TestGitOnlyGitops replace push/pull ban |
| T004 | 1 | Review queries in internal/synth | T001 | sonnet | done | internal/synth review.go + test |
| T005 | 1 | review command | T004 | sonnet | done | cmd_review.go + test; agent_test.go schema map adapted |
| T006 | 2 | collide role and embedded prompt | T000 | sonnet | done | collide role, prompt, init test, fakeModel |
| T007 | 2 | Provider resolution by role and medium | T001 T006 | sonnet | done | Effective/ForRole by medium, Deps.ProviderFor+AuthorFor |
| T008 | 2 | stats tokens for every role | T006 | haiku | done | stats.go + test; collide tokens row |
| T009 | 3 | Exported scrubbed-env argv runner | T000 | opus | done | exec.go RunArgv+IsNotFound, TestRunArgv |
| T010 | 3 | PDF intake via pdf.command | T001 T009 | opus | done | intake ReadWith + PDF via RunArgv, tests adapted |
| T011 | 3 | Batch add --from | T007 T010 | opus | done | pipeline/batch.go, add.go stages, cmd_add --from, batch tests |
| T012 | 3 | Wildcard lane | T011 | opus | done | pipeline/wildcard.go, report.Wildcard, tests |
| T013 | 3 | Per-medium cap runs_per_day | T002 T011 | sonnet | done | runs_per_day cap; counts only mediums with a limit (deviation) |
| T014 | 3 | Per-medium cap tokens_per_run | T002 T011 | sonnet | done | tokens_per_run meter in add/batch; batch stage order now per-entry |
| T015 | 4 | Collide pair selection | T001 | sonnet | done | synth/collide.go Pool/Select/ExistingPairs + test |
| T016 | 4 | collide command and proposals | T007 T015 | opus | done | cmd_collide.go, synth/propose.go, report.Collision, tests |
| T017 | 4 | keep command | T000 | sonnet | done | cmd_keep.go + tests; update shares commitDoc |
| T018 | 5 | gitops pull, push, behind | T003 | opus | done | gitops pull/push/behind + tests; pulled = old..@{u} when upstream (deviation, correct with local commits) |
| T019 | 5 | Pull before write in the run wrapper | T001 T002 T018 | opus | done | cli/sync.go prepareRun, --no-pull, git.pulled; no extra pre-pull dirty check (autostash covers it) |
| T020 | 5 | Push after commit | T019 | opus | done | --push/--no-push, runPush in sync.go, gitops Ahead; push skipped under no-commit |
| T021 | 5 | sync command, --sync and dry-run BEHIND | T020 | opus | done | cmd_sync.go, --sync, dry-run behind/BEHIND warning |
| T022 | 6 | skill guide v2 | T005 T011 T016 T017 T021 | haiku | done | skill guide v2 (47 lines) + TestSkillV2 |
| T023 | 6 | README v2 update | T005 T007 T010 T011 T012 T013 T014 T016 T017 T019 T020 T021 T022 | sonnet | done | README v2 sections + TestReadmeV2Mentions |
| T024 | 6 | CLAUDE.md architecture notes v2 | T004 T009 T010 T018 | haiku | done | CLAUDE.md architecture notes v2 |
| T025 | 6 | Definition of Done | T001 T002 T003 T004 T005 T006 T007 T008 T009 T010 T011 T012 T013 T014 T015 T016 T017 T018 T019 T020 T021 T022 T023 T024 | supervisor | done | DoD lines 1-13 pass |

Status values: `todo` · `done` · `blocked` · `superseded`. No pipe characters inside cells.

## Execution Protocol

Run with the `loop-coding` skill (`/loop-coding v2`). The authoritative procedure
(round limit, commit format, Definition-of-Done repair) is in
`.claude/skills/loop-coding/SKILL.md`; in short, per iteration:
1. Select the first `todo` row whose dependencies are all `done`.
2. Read `.harness/versions/v2/tasks/<ID>.md`; dispatch one fresh worker with the row's `Model` and a brief made of the
   Standing constraints, the task's Brief and the path of its verification script.
3. The supervisor runs `.harness/verify/v2/<ID>.sh` itself (incl. its negative checks); never trust
   the worker's claim.
4. Pass → one commit with the work and this file's update. Fail → feedback; still failing
   after 3 rounds → `blocked`.
5. Nothing selectable → run the Global Definition of Done of `.harness/versions/v2/plan.md`; done when it passes.
