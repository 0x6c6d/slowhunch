# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

<Fill in once code exists: build / test / lint commands, architecture notes. Until then: the spec is `.harness/harness.md`.>

## Delegation workflow: supervise, don't implement

For non-trivial implementation work, use the supervisor/worker loop (`/loop-coding`, full procedure in `.claude/skills/loop-coding/SKILL.md`) instead of implementing directly:

- Project flow, per version `vN`: spec (`.harness/harness.md` for v1, `.harness/versions/vN/spec.md` for later versions, written with `.harness/prompts/new-version.md`) → `.harness/prompts/generate-plan.md` → `.harness/versions/vN/plan.md` (+ the core `.harness/Plan.md`) → `.harness/prompts/generate-tasks.md` → `.harness/versions/vN/Tasks.md` (small index) + `.harness/versions/vN/tasks/<ID>.md` (brief) + `.harness/verify/vN/<ID>.sh` (verification) → loop runs the index row by row and reads a task file only when it dispatches that task; it never reads the folders of other versions.
- Done = a verification script that exits 0, never "looks right"; workers never touch the spec, prompts, skill or this section (the supervisor checks the diff).
- One fresh worker per task, `model` per task (haiku / sonnet / opus); the supervisor verifies and commits itself and never trusts the worker's report.
- Fail → concrete feedback to the same worker; still failing after 3 rounds → `blocked` and ask the user.
