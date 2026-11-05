# slowhunch: not in v1 (input for later versions)

> Notes for `.harness/prompts/new-version.md`. Everything here was in `prompt.md` and was cut from v1
> on 2026-11-05. Detail lives in `prompt.md`; this file only lists what moves and why it waits.
> The `anthropic` provider is **dropped completely**: not in v1, not planned for v2. Anthropic models
> are reachable through any `openai-compatible` endpoint (e.g. OpenRouter) or through `exec`.

## Candidate v2: synthesis

- **`slowhunch collide`**: weekly combination of distant atoms or hunches into a `proposals/` file
  with a cheapest test. Aesthetic atoms weighted up; 25% of every run reserved for `asserts` x `reads`
  across media; capped in config; the most expensive command. Proposals stay proposals until the owner
  promotes them.
- **`slowhunch review`**: monthly pass over stale hunches, orphans, unplaced aesthetic atoms.
- **Wildcard lane** in `place` (10% share of near-miss rejections listed in the report) and the
  `collisions` / `wildcards` report keys, if v1 only emits them empty.
- **Topics** append flow ("keep that" appends a dated, quoted, linked block to the nearest topic), if
  v1 only creates the folder and type.

## Candidate v2: git sync

- **Pull before every write**: `git pull --rebase --autostash`, index rebuild after the pull,
  `PULL_FAILED`, `BEHIND` on dry-run, `git.pull`, `git.require_pull`, `--no-pull`, `--sync` on
  read-only commands.
- **Push after the commit** with a plain `git push` (never force), `PUSH_FAILED` retried by the next
  run, `git.push`, `git.remote`, `git.branch`, `--no-push`.
- **`slowhunch sync`**: pull then push on demand.
- **Rebase conflict handling**: abort, leave the vault untouched, stop with `CONFLICT` (exit 4) naming
  the files. v1 keeps only the local-commit conflict (uncommitted foreign changes) if it needs one.
- Two-machine success criterion: a change pushed on one machine is pulled before the next write on
  the other.

## Candidate v2: input and providers

- **Batch input**: `slowhunch add --from batch.txt`, one entry per line
  (`<file>` or `<file>|<url>|<title>|<author>`).
- **PDF text extraction** for `add <file.pdf>` (plain text only, no OCR). In v1 a `.pdf` fails with
  `UNSUPPORTED`. Stdlib has no PDF reader, so this needs either a justified third-party package or an
  optional external tool (e.g. `pdftotext`) behind an `exec`-style hook; decide in the v2 plan.
- **Per-role provider split** beyond what v1 needs (gate, extract, place, collide on different
  providers and models), and `medium`-specific model choice.
- **Provider cost caps** per `medium` and runs per day per `medium`, if v1 only has per-input and
  per-run atom caps.
- **`send_policy` levels** beyond `excerpt_only`, including `local_only` for marked folders.

## Candidate v2: documentation set

v1 ships only `README.md` (+ `LICENSE`). The rest moves to v2:

- `docs/setup.md`, `docs/inputs.md`, `docs/concepts.md`, `docs/vault-format.md`,
  `docs/providers.md`, `docs/agent-guide.md`, `docs/prompts.md`, `docs/security.md`,
  `docs/management.md`, `CONTRIBUTING.md`.
- `examples/vault/` with fake data, and the rule that every command shown in the docs is exercised by a
  test.
- Scheduling docs (systemd and cron).

## Later / not scheduled

- Better search for verse (TF-IDF is weak on short lines): small optional binary index, then maybe
  embeddings. First thing to revisit after a month of use.
- `slowhunch prompts diff <n>` if v1 only versions prompts without a diff command.
- MCP server wrapper around the CLI.
- Migration note for importing an existing vault.

## Explicitly never (carried over from `prompt.md`)

Web UI; any content fetching by slowhunch itself (URLs, web search, platform sites, feeds, scheduled
ingest); an `anthropic` provider; crawling or following outlinks; logging in or storing credentials;
downloading or storing media; vision or audio models; OCR, scores or chords; multi-user sync; a daemon.
