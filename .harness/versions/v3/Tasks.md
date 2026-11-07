# Tasks v3: slowhunch

_Generated from .harness/versions/v3/plan.md. This file is the loop's persistent state (status and notes
only): update it in place, never regenerate it during a build. Briefs live in `.harness/versions/v3/tasks/<ID>.md`,
verifications in `.harness/verify/v3/<ID>.sh`. IDs are unique within this version._

## Standing constraints

The supervisor prepends these, verbatim, to every worker brief:

- `make check` must pass for the whole repo.
- Protected (never modify): the whole `.harness/` folder, `.claude/`, the "Delegation workflow" section of `CLAUDE.md`, `.yipyip/`. Do nothing outside your task (the repo-root `README.md` is written only by the README task).
- Go standard library only; no new packages; no network; do not commit (the supervisor commits).

## Task List

| ID | Phase | Title | Depends-on | Model | Status | Notes |
|----|-------|-------|------------|-------|--------|-------|
| T000 | 0 | Preflight | — | supervisor | todo | |
| T001 | 1 | Wikilink strip in Normalize + unit tests | T000 | sonnet | todo | |
| T002 | 2 | E2E: add then verify clean, raw quote rejected | T001 | sonnet | todo | |
| T003 | 2 | Legacy vault unchanged + pipe round trip tests | T001 | sonnet | todo | |
| T004 | 3 | README update | T001 | sonnet | todo | |
| T005 | 3 | CLAUDE.md architecture note | T001 | haiku | todo | |
| T006 | 3 | Definition of Done | T001, T002, T003, T004, T005 | supervisor | todo | |

Status values: `todo` · `done` · `blocked` · `superseded`. No pipe characters inside cells.

## Execution Protocol

Run with the `loop-coding` skill (`/loop-coding v3`). The authoritative procedure
(round limit, commit format, Definition-of-Done repair) is in
`.claude/skills/loop-coding/SKILL.md`; in short, per iteration:
1. Select the first `todo` row whose dependencies are all `done`.
2. Read `.harness/versions/v3/tasks/<ID>.md`; dispatch one fresh worker with the row's `Model` and a brief made of the
   Standing constraints, the task's Brief and the path of its verification script.
3. The supervisor runs `.harness/verify/v3/<ID>.sh` itself (incl. its negative checks); never trust
   the worker's claim.
4. Pass → one commit with the work and this file's update. Fail → feedback; still failing
   after 3 rounds → `blocked`.
5. Nothing selectable → run the Global Definition of Done of `.harness/versions/v3/plan.md`; done when it passes.
