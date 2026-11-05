# slowhunch (v2 spec, delta on `.harness/harness.md`)

## 1. Goal
v2 adds synthesis (`collide`, `review`, a wildcard lane, `keep`), richer input and provider control (batch `add`, PDF text via an optional external tool, per-role and per-medium providers, per-medium caps) and git sync (pull before write, optional push, `sync`). Everything not named here keeps v1 behavior.

## 2. New requirements
**Synthesis**
- **R25.** `slowhunch collide [--limit N] [--dry-run]` picks pairs from atoms plus `open|warming` hunches whose TF-IDF cosine is <= `collide.max_cosine` (default 0.15); scores are multiplied by `collide.aesthetic_weight` (default 2.0) when an item is aesthetic; at most `collide.max_pairs` (default 6, `--limit` can only lower it) pairs per run; at least `floor(0.25 * pairs)` of them (and one when pairs >= 4) are an `asserts` atom x a `reads` atom from different media. Selection is deterministic (sorted by score, then ids). A cap that bites is reported.
- **R26.** Per pair, one model call with role `collide` (embedded prompt `collide.md`, versioned like the others, same fencing and `excerpt_only` rules). Reply JSON `{idea, cheapest_test, keep}`; code validates ids and text. A kept pair becomes `proposals/<kebab idea>.md` (type `proposal`, `items` = the two ids, `idea`, `cheapest_test`, `status: open`, `bears_on` = hunches among the items). Collide never writes notes or hunches and never promotes a proposal. A pair already listed in some proposal's `items` is skipped, so a re-run on an unchanged vault writes nothing. One commit per run; the report's `collisions` key lists `{items, proposal}` and `counts` the pairs tried and kept.
- **R27.** `slowhunch review [--stale-days N]` is read-only and model-free. It lists: stale hunches (`open|warming` with no evidence entry and no `revisit` change in `review.stale_days`, default 60 days), orphans (notes with no backlinks and no outgoing links), and unplaced aesthetic atoms (aesthetic notes with no `rereads`, no echo link and no backlinks). `--json` result `{stale_hunches, orphans, unplaced}`; exit 0.
- **R28.** Wildcard lane: every gate-rejected item of a run is listed in the report's `wildcards` key (`{item, reason, kept}`), never dropped silently. `floor(place.wildcard_share * rejected)` of them (default share 0.10; chosen by lowest sha256 of the item's `text_hash`) are written anyway as normal atoms tagged `wildcard` (`kept: true`). A single rejected item is therefore never kept.
- **R29.** `slowhunch keep "<text>" --ids a,b [--topic ID]` appends a block (`### YYYY-MM-DD`, the text, then `[[a]] [[b]]`) to the body of a topic through the existing write path with rev check, one commit. Without `--topic` the nearest topic by TF-IDF over title and body is used; no topics or an unknown id fails `NOT_FOUND`, a secret-like text `BAD_INPUT`, text over 2,000 characters `TOO_LONG`. No other file is created.

**Input and providers**
- **R30.** `slowhunch add --from batch.txt` runs the `add` pipeline per line: `<file>` or `<file>|<url>|<title>|<author>` (blank lines and lines starting with `#` ignored; at most 50 entries, else `TOO_LONG`; `--as`, `--note`, `--hunch`, `-force` apply to all entries). One run id, one commit; entries fail independently (reported in `errors` with the line number) and every other entry is written. Result `{run_id, items: [<report per entry>], counts, git}`; item reports are saved as `.slowhunch/reports/<run-id>-<n>.json`.
- **R31.** PDF: `add file.pdf` extracts plain text by running `pdf.command` from config (argv, no shell, scrubbed environment as in R19, 60 s timeout, `{file}` replaced by the path, stdout is the text; default `["pdftotext", "-layout", "{file}", "-"]`). Output goes through the normal checks (12,000 characters, normalization, hashing, medium inferred `article` unless `--as`). Tool missing, failing or output empty: `UNSUPPORTED` with hint "install pdftotext or set pdf.command". No OCR.
- **R32.** Provider selection per role and medium: roles `gate extract place collide`; precedence `media.<medium>.<role>` > `roles.<role>` > `provider`, each overriding only the fields it sets. `slowhunch stats` reports tokens per role (adds `collide`).
- **R33.** Caps by medium in config `caps.by_medium.<medium>`: `runs_per_day` (count of runs that called a provider, local date, kept in `.slowhunch/state.json`) and `tokens_per_run` (stop calling providers once the run's tokens reach it). Absent or 0 = unlimited. A run refused for `runs_per_day` fails `CAP_REACHED` (exit 2, nothing sent or written); hitting `tokens_per_run` marks the remaining items `CAP_REACHED` in `errors` and writes the finished ones.

**Git sync**
- **R34.** Pull before write: with a remote and an upstream, a vault-changing run first runs `git pull --rebase --autostash` (never a merge commit), then rebuilds the in-memory index. `git.pull` default true; `--no-pull` skips; no remote = no-op. The `git` report key carries `pulled` (commit count).
- **R35.** Push after commit: `git.push` default **false**; when true (or `--push`) and a remote exists, a plain `git push` follows the run's commit. `git.remote` (default `origin`), `git.branch` (default current). A failed push never undoes the commit: report `PUSH_FAILED` with a short hint; the next run that pushes sends everything pending. `--no-push` skips. `git` report key carries `pushed`.
- **R36.** `slowhunch sync` pulls then pushes on demand regardless of `git.push` (honors `--no-pull`/`--no-push`), writes no files, reports `{pulled, pushed}`. No remote: `NOT_FOUND` with hint.
- **R37.** `--dry-run` never pulls or pushes; it reports `BEHIND` (count of upstream commits not yet local, from the already-fetched upstream ref, no network) and leaves the vault byte-identical. Read-only commands (`search get list stats review verify lint`) do not pull unless called with `--sync`.
- **R38.** Failures: a rebase conflict aborts the rebase, restores the vault exactly as before, writes nothing and fails `CONFLICT` (exit 4) naming the files; an unreachable remote reports `PULL_FAILED` and the run continues locally, unless `git.require_pull` (default false) is true, then it stops with `PULL_FAILED` (exit 1) before writing. slowhunch never resolves a conflict and never picks a side.
- **R39.** Safety: every git call stays in `internal/gitops`; `push --force`, `--force-with-lease`, `-f`, `+<refspec>`, `reset --hard`, `filter-branch` and any history rewrite never appear in the code.

**Docs**
- **R40.** `slowhunch skill` lists the new commands and still fits 60 lines; `README.md` documents `collide`, `review`, `keep`, `add --from`, PDF setup, the provider/medium override and caps tables, and git sync (`git.pull`, `git.push`, `sync`), and its Quick start test still passes.

## 3. Changed requirements
- R7: was `.pdf` always `UNSUPPORTED`, no batch → now `.pdf` per R31 and `--from` per R30.
- R12: was `wildcards` and `collisions` always empty arrays → now filled by R28 and R26; `git` key adds `pulled`, `pushed`.
- R17: was ten codes → adds `CAP_REACHED`, `PULL_FAILED`, `PUSH_FAILED`, `BEHIND`.
- R18: was roles `gate extract place` → adds `collide` and `media` overrides (R32).
- R21: was "v1 never pulls and never pushes" → pull per R34, optional push per R35; commit rules otherwise unchanged.
- R22: was no network except provider `base_url` → also the `git` binary talking to the vault's configured remote, and `pdf.command`; no other host.
- R23: was global caps only → plus per-medium caps (R33).

## 4. Removed requirements
- R21: the clause "v1 never pulls and never pushes" and the v1 policy tests that ban the words `git push`/`git pull`/`"push"`/`"pull"` outside tests; replaced by R39.

## 5. Constraints & environment changes
- New optional tool: `pdftotext` (poppler); not required to build or test (`command -v pdftotext` is informational only; tests use a test-binary helper as `pdf.command`).
- New live system: a git remote. Tests use a local bare repo under `t.TempDir()` as `origin` (file path); no network host, no credentials, never this repository's own remote.
- `internal/gitops` may run `pull --rebase --autostash`, `rebase --abort`, `push`, `rev-list` (no `fetch`); all other v1 constraints (stdlib only, no new secrets, argv only) hold.
- New config keys: `collide.*`, `review.stale_days`, `place.wildcard_share`, `pdf.command`, `media`, `caps.by_medium`, `git.pull|require_pull|push|remote|branch`.

## 6. Acceptance criteria
- [ ] `make check` exits 0 and the v1 tests still pass. [R1-R24]
- [ ] Test (fake provider, seeded vault): `collide` picks only pairs with cosine <= 0.15, includes the reserved `asserts` x `reads` cross-medium pairs, weights aesthetic items, respects `max_pairs` and reports the cap; kept pairs write valid proposals with `items`/`bears_on`; a second run writes nothing; no notes or hunches change. [R25, R26]
- [ ] Test: `review` on a seeded vault lists exactly the expected stale hunches, orphans and unplaced aesthetic atoms and changes nothing (tree hash). [R27]
- [ ] Test: a batch with 10 gate-rejected items keeps exactly one tagged `wildcard`; a single rejection keeps none; all appear in `wildcards`. [R28]
- [ ] Test: `keep` appends a dated block with `[[id]]` links to the named topic, or to the nearest topic when `--topic` is absent, with a correct rev; missing ids, no topic, secret text and 2,001 characters fail as specified, writing nothing. [R29]
- [ ] Test: `add --from` with 3 good, 1 `.pdf`-failing and 1 oversize entry writes the 3, reports 2 errors with line numbers, makes one commit; 51 entries fail `TOO_LONG`. [R30]
- [ ] Test: with a helper-binary `pdf.command`, a `.pdf` is added as `article`; a missing or failing command gives `UNSUPPORTED` with the hint; the command gets a scrubbed environment and no shell. [R31, R22]
- [ ] Test: provider resolution follows `media.<medium>.<role>` > `roles.<role>` > `provider` field by field; `stats` shows `collide` tokens. [R32]
- [ ] Test: `runs_per_day` 1 allows the first run and refuses the second with `CAP_REACHED`, nothing written; `tokens_per_run` stops mid-batch with the rest marked. [R33]
- [ ] Test (local bare remote, two clones): a change committed and pushed (`git.push` true) from clone A is pulled before the next write in clone B and visible to placement; `git.push` false pushes nothing; `--no-pull`/`--no-push` honored; `pulled`/`pushed` reported. [R34, R35]
- [ ] Test: `sync` pulls then pushes and writes no files; without a remote it fails `NOT_FOUND`. [R36]
- [ ] Test: `--dry-run` with the remote ahead reports `BEHIND` and leaves the tree byte-identical; `search` does not pull, `search --sync` does. [R37]
- [ ] Test: diverging edits to one file make the run fail `CONFLICT` (exit 4) with the rebase aborted and files unchanged; with the remote removed the run continues with `PULL_FAILED`, and `git.require_pull` true stops it before writing; a failed push is reported and retried by the next run. [R38, R35]
- [ ] Test: source scan finds no forbidden force/rewrite forms outside tests, and every `git` call site is inside `internal/gitops`. [R39]
- [ ] `skill` output is <= 60 lines and names the new commands; the README mentions each new command and config key (grep) and its Quick start test passes. [R40]

## 7. Out of scope
`docs/` set, `CONTRIBUTING.md`, `examples/`, scheduling docs; `local_only` or any other `send_policy` level; MCP server; embeddings or better verse search; `prompts diff`; vault import; OCR, vision, audio; a scheduler or daemon (owners run `collide` and `review` from their own cron); automatic conflict resolution; third-party Go packages.
