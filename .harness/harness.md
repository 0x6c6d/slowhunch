<!-- SPEC-PLACEHOLDER: replace this whole file with the project spec -->
# <Projektname>

Platzhalter. Diese Datei wird zur **Spec** des Projekts, aus der der Loop Plan und
Tasks erzeugt. Sie ist die Baseline (v1); spätere Versionen bekommen je eine kleine Delta-Spec
unter `.harness/versions/vN/spec.md` (Prompt `.harness/prompts/new-version.md`). Erstellen:

1. Claude Code im Projektordner starten.
2. Den Prompt unten einfügen, `[...]` am Ende ausfüllen.
3. Claude stellt kurze Rückfragen und überschreibt dann diese Datei mit der Spec.
4. Spec prüfen, besonders die Abschnitte *Environment* und *Acceptance criteria* sowie alle
   Stellen mit „(assumption)". Diese Datei ist nur für den Loop; die öffentliche `README.md`
   des Projekts entsteht erst im letzten Task.

```
You are a senior product engineer writing a build spec for an autonomous coding loop.
The loop never asks questions. A planner turns this spec into .harness/Plan.md (core plan: architecture,
commands) and .harness/versions/v1/plan.md (phases, assumptions, definition of done), then into
.harness/versions/v1/Tasks.md (small tasks, each with a verification command). Later versions are added as
small delta specs on top of this file, so this file is the baseline: Tech stack, Constraints and
Environment must hold for every later version. Whatever the spec leaves open becomes a guess, so be
concrete.

First interview me: ask short questions, one at a time (at most 10). With every question,
propose a default I can accept with "ok". Cover: what to build and for whom, core features, the
main flow, the tech stack (language, framework, packages), constraints and out-of-scope, the
environment (installed tools, credentials, external services or live systems the build may
touch during development), how users install, run and configure it, deliverables, and how
"done" is checked. Look at the repository
yourself instead of asking what you can see.

Then overwrite .harness/harness.md with the spec, in exactly these sections and this order:
1. Overview: what and why, 3–5 sentences
2. Core requirements: numbered R1, R2, ...; each one testable
3. Flow: the main user/data flow, step by step
4. Tech stack: concrete languages, frameworks, packages with versions (real package names)
5. Constraints & out of scope: security, data/privacy, legal, performance, what must NOT be
   built, and files that must not be touched (the whole .harness/ folder, .claude/ and the
   "Delegation workflow" section of CLAUDE.md; .harness/ is managed by the loop). Label security-,
   data-, legal- and money-related items clearly: the planner treats them as high-risk.
6. Environment: where the code lives (repo root; no new nested project folder, no git init),
   required tools each with a check command that exits 0, secrets (env var names only, never
   values), and the only live systems or test targets the build may touch (everything else is
   off limits)
7. Deliverables: files and artifacts that must exist at the end (code, tests, docs). Always
   include a public-facing README.md at the repo root (not part of .harness/): what the project
   is, installation, usage, and, where the project has user-facing config options, customization;
   it is written by the last task. If you know the config surface (options, defaults, env var
   names), list it here; if there is none or it is unclear, say so and leave it out.
8. Acceptance criteria: checkboxes, each verifiable by a command or test with an expected
   result, and each naming the requirement(s) it covers (e.g. "[R3]"); every requirement is
   covered at least once. Include one command that runs everything (tests + lint + build) and
   exits 0 only if the project is healthy
9. Nice-to-haves: explicitly not part of v1 (candidates for later versions)

Rules: no vague words ("fast", "user-friendly") without a measurable number; every requirement
testable; mark your own assumptions with "(assumption)"; write in English; remove the
SPEC-PLACEHOLDER line. Write only .harness/harness.md: no code, no .harness/Plan.md, no .harness/versions/. Finish with
the list of your assumptions for me to confirm; after I confirm, commit .harness/harness.md as "spec".

My project idea: [describe the project in a few sentences]
```
