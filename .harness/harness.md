# slowhunch (v1 spec)

## 1. Overview

slowhunch is a Go command-line program plus a vault format that turns a personal knowledge base into a
**commonplace book optimized for AI agents**. An agent hands it text (articles, papers, poems, lyrics,
quotations, descriptions of paintings or scenes, the owner's own notes); slowhunch gates, extracts and
places it as small linked **atoms** with verbatim evidence, attaches them to long-running **hunches**
(open questions that collect dated evidence), and reports contradictions first. The vault is plain
Markdown with frontmatter in a git repo, has no prose layer and no UI, and every command has `--json`
output so any agent can build on it. slowhunch itself never fetches content from the network: the agent
reads links and searches with its own tools and passes the text in. Design inspiration is Steven
Johnson, *Where Good Ideas Come From*; there is no evidence the patterns improve idea quality, so
`slowhunch stats` is part of the product.

Scope of this baseline (v1): vault, `add` pipeline, CRUD/links/hunches, search, `verify`/`lint`/`stats`/
`skill`, two providers, local git commit per run, README. Everything else is listed in section 9 and in
`prompt_v2.md` (repo root, informational only, not a spec).

## 2. Core requirements

**Vault and files**

- **R1.** `slowhunch init <dir>` creates a vault: folders `notes/ hunches/ sources/ topics/ proposals/
  inbox/ archive/ .slowhunch/`, `.slowhunch/config.json` (defaults in section 7), prompt files in
  `.slowhunch/prompts/`, `AGENTS.md` and `CLAUDE.md` with identical short text (real files; text says:
  run `slowhunch skill` at session start, run `slowhunch verify` before and after any batch, write only
  through the `slowhunch` CLI), and a `.gitignore` containing `.slowhunch/cache/`. It runs `git init` if
  `<dir>` is not a repo and makes the first commit. It never overwrites an existing `AGENTS.md` or
  `CLAUDE.md`: it reports code `EXISTS` for that file and leaves it alone. If `git` is missing, init
  still succeeds and the report carries `GIT_MISSING`.
- **R2.** Every stored file is Markdown with YAML-style frontmatter and one of five types: `note` (atom),
  `source`, `hunch`, `proposal`, `topic`. Common frontmatter: `id` (kebab-case slug equal to the file
  name without `.md`), `type`, `created`, `updated` (`YYYY-MM-DD`, local time), `author` (`human` or
  `agent:<provider>/<model>`), `rev`, `tags`, `bears_on` (list of hunch or proposal ids). Type-specific
  fields:
  - note: `claim`, `evidence`, `stance`, `locator`, `source`, `status`, `unlocks`, `repurpose`,
    `confidence` (only for `asserts`), `rereads`, `rendering`, `translates`.
  - hunch: `question`, `state` (`open|warming|tested|dropped`), `next_test`, `evidence` list of
    `{date, note, relation}` with relation `supports|weakens`, `revisit`.
  - source: `url`, `medium`, `title`, `author`, `published`, `atoms` (ids taken), `translates`,
    `owner_note`, `text_hash`.
  - proposal: `items`, `idea`, `cheapest_test`, `status`. Topic: `title` plus appended blocks.
  - v1 creates and validates `proposal` and `topic` files via the generic CRUD only; no command generates
    proposals and no command appends topic blocks except `update --append` (see R14).
- **R3.** An atom file body has exactly two parts: the `claim` as one line, then the excerpt as a
  blockquote. The frontmatter `evidence` equals the blockquote text with the `> ` prefix stripped. No
  other prose. `stance` is `asserts|crafts|reads`; `medium` is
  `paper|article|blog|post|thread|page|code|book|own|poem|lyric|prose|quote|film|art|talk|other`.
- **R4.** Ids: source `<medium>-<author>-<title>-<short>` (lowercase ASCII kebab-case, each part cut to 24
  characters, `<short>` = first 4 hex of the sha256 of the normalized intake text, missing author =
  `anonymous`, `own` sources use `own-<title>-<short>`); atom `<stance>-<source-slug-without-short>-
  <locator-slug>` (e.g. `crafts-larkin-trees-lines-1-2`); hunch and proposal ids are a kebab slug of the
  question or idea, at most 60 characters. A colliding id gets suffix `-2`, `-3`, .... Re-wording a claim
  never changes an id.
- **R5.** `rev` is the first 6 hex characters of the sha256 of the file's canonical bytes excluding the
  `rev` line. Every update checks the caller's `rev`; a stale `rev` fails with code `CONFLICT_REV` and
  returns the current content. Writes are atomic (temp file plus rename), confined to the vault with a
  symlink-resolved path check, and logged to `.slowhunch/audit.log`. Excerpts (`evidence`) are never
  rewritten; corrections are a new atom or a field patch.
- **R6.** Links are `[[id]]`, one direction only; backlinks are computed. `locator` is a closed set of
  shapes: `line N`, `lines N-M`, `stanza N`, `verse N`, `comment N`, `section <name>`,
  `timecode 1:23-1:31`, `region x1,y1,x2,y2`.

**`add` pipeline**

- **R7.** `slowhunch add <text|file|-> [--as <medium>] [--url U] [--title T] [--author A] [--note N]
  [--hunch ID] [--force] [--dry-run]` reads one text input (argument, stdin via `-`, or a `.txt`/`.md`
  file). Any other file type, including `.pdf`, fails with `UNSUPPORTED`. Input longer than 12,000
  characters fails with `TOO_LONG` (no truncation, no chunking). Empty input fails with `BAD_INPUT`.
  `--as poem|lyric|film` requires `--title`; `--as quote` requires `--author`; unknown authorship is
  written `anonymous`, never guessed; `--as art|film` with no text fails `UNSUPPORTED` with the hint
  "describe or quote the image or scene; there is no vision or audio model". `--url` is a citation only
  and is never requested. Without `--as`, the medium is inferred from file extension and length only
  (`.md`/`.txt` long = `article`, short with line breaks = `other`) and the report states the inferred
  value.
- **R8.** Intake normalization (stdlib, no model): whitespace normalized, line breaks kept, curly quotes,
  em dashes and non-breaking spaces normalized once; for `--as poem|lyric|prose|quote|film` every line is
  numbered; for `--as code` lines are numbered and whitespace is untouched. The normalized text is
  written to `inbox/<source-id>.txt`, immutable once written, and is the ground truth for evidence and
  locator checks. The same `text_hash` returns the existing source and writes no second one.
- **R9.** Gate (model call, one per item, fails closed on provider error: item rejected and counted, never
  written). The question depends on `medium`: factual (`paper article blog page post thread code talk
  book`), aesthetic (`poem lyric prose quote film art`), or owner-steered (`other`, `own`, or any input
  with `--note` or `--hunch`: always kept). `--force` bypasses the gate.
- **R10.** Extract (model call, one per item): factual input yields 1 to 5 atoms, aesthetic 1 to 3, `own`
  1 to 5 (stance `asserts` with low `confidence` by default, or `reads` when it interprets). Rules enforced
  in code, never by the prompt: `evidence` must be an exact substring of the stored normalized text or the
  atom is dropped and counted with its reason; `locator` required for aesthetic media, `thread`
  (`comment N`) and `code` (`lines N-M`) and validated against the stored text; `confidence` only on
  `asserts`; foreign-language `evidence` stays in the original language with the gloss in `rendering`;
  `art` and `film` evidence is the owner's description or quoted lines. Aesthetic atoms are never
  paraphrases: the claim is one line and the excerpt is the cut line(s).
- **R11.** Place (model call per new atom, never batched): near neighbours are the top 5 atoms by TF-IDF
  cosine similarity, plus one far neighbour (the best-scoring atom of the other class, factual vs
  aesthetic; none if there is no such atom); each neighbour excerpt is capped at 300 characters. The model
  returns a verdict `duplicate|new|extends|contradicts`; **code decides** what is written:
  `duplicate` needs cosine >= 0.80 else becomes `extends`; `contradicts` needs a shared tag or cosine >=
  0.20 else becomes `extends`; `reads` atoms are never duplicates and instead get a `rereads` link to the
  existing reading; an aesthetic atom filed `new` with best cosine in [0.20, 0.80) is also recorded as an
  **echo**. These numbers live in `config.json`.
- **R12.** Write and report: new atom plus thin source file; a duplicate appends a dated corroboration
  line to the existing atom (never for `reads`) and writes no new file; a contradiction writes a flagged
  atom linked to both sides and adds hunch evidence (relation `weakens`); `--hunch` and matching open
  hunches get a dated evidence entry. The report is one JSON object printed to stdout and saved to
  `.slowhunch/reports/<run-id>.json` with the keys in this order: `contradictions`, `hunches_moved`,
  `atoms`, `corroborations`, `echoes`, `wildcards`, `collisions`, `counts`, `errors`, `cost`, `git`;
  `wildcards` and `collisions` are empty arrays in v1; every `[[id]]` in a report exists on disk when the
  report prints; `cost` is tokens per role. `--dry-run` prints the plan and leaves the vault
  byte-identical (no files, no commit, no audit line).
- **R13.** Idempotent: running the same `add` twice writes nothing the second time. A crash mid-run leaves
  its files uncommitted and the resumed run commits them together.

**Agent interface**

- **R14.** CRUD and graph: `create`, `get`, `update` (field patch with `--rev`, or `--append` for body
  append on `topics/` only), `delete` (soft, moves to `archive/`; `--hard --yes` removes), `list` with
  filters (`--type --medium --stance --tag --hunch --state`), `link`, `unlink`, `backlinks <id>`,
  `graph <id> --depth N`. Hunches: `hunch add`, `hunch evidence <id> --note --relation`, `hunch state
  <id> <state>`, `hunch due` (revisit date <= today). `source facts <id>` prints the stored text with
  line numbers.
- **R15.** `search "<query>"` is one ranked call (stdlib TF-IDF) with `--medium --stance --tag --hunch
  --budget <tokens> --limit N`, returning atoms ranked by score, recency and hops, each with quote,
  locator and source. A `reads` hit includes the atom it `rereads`; a hit whose source `translates`
  another source includes the counterpart.
- **R16.** `lint` fails (exit 3, one `--json` failure per item, nothing written) on: an aesthetic atom
  without `locator`, a `confidence` on a non-`asserts` atom, a `rendering` where `evidence` belongs, a
  `reads` atom with no `rereads` link to the reading it repeats. `verify` re-derives the index from files
  and checks every `evidence` substring against `inbox/`, every `locator` range, every `[[id]]` link and
  every `rev`; it reports each problem by `id`, line and code, changes nothing, and exits 3 on any
  failure. `stats --json` reports atoms by `medium` and `stance`, the `asserts`/`reads` ratio, hunches by
  state, contradictions, echoes, and tokens per role. `skill` prints a guide (data model, rules, example
  call sequences, session shape) that fits in 60 lines.
- **R17.** Every command supports `--json` with a versioned schema (`"schema": 1`). Errors are
  `{code, message, hint}`; codes used: `NOT_FOUND`, `BAD_INPUT`, `UNSUPPORTED`, `TOO_LONG`, `EXISTS`,
  `CONFLICT`, `CONFLICT_REV`, `GIT_MISSING`, `PROVIDER_ERROR`, `LINT_FAILED`. Exit codes: 0 ok, 1
  config/fatal, 2 usage, 3 lint/verify failure, 4 conflict.

**Providers**

- **R18.** One Go interface `Provider` with a registry; `config.json` selects provider, `base_url`,
  `model`, `api_key_env`. Roles `gate`, `extract`, `place` use the same provider in v1; the config schema
  allows a per-role override. Built-ins: `openai-compatible` and `exec`; there is no other provider.
- **R19.** `openai-compatible` calls `POST {base_url}/chat/completions` with `response_format
  {"type":"json_object"}`, temperature 0, 60 s timeout, reads tokens from `usage`, retries once on 429/5xx
  with backoff. `exec` runs the argv in config (no shell) with JSON `{role, medium, system, user,
  max_tokens}` on stdin and expects `{text, tokens_in, tokens_out}` on stdout, 120 s timeout, environment
  scrubbed to `PATH` plus the env var named in config.
- **R20.** Model output is requested as JSON, validated in Go, retried once with a repair prompt, then
  counted as a per-item error. Model output is never trusted as instructions or file paths. Prompts for
  gate, extract and place are files with a `version` field in `.slowhunch/prompts/` (embedded in the
  binary, written by `init`); the version is recorded in `state.json` and in every audit line.

**Git**

- **R21.** Every vault-changing command (`add`, `create`, `update`, `delete`, `link`, `unlink`, `hunch
  ...`, `init`) ends with one git commit per run containing exactly the files that run wrote or moved
  (never `git add -A`). If a run would touch a file with uncommitted changes it did not make, it stops
  with `CONFLICT` (exit 4) before writing. Commit message: `slowhunch: <command> <ids> (<counts>)`, run id
  and prompt versions in the body. The commit hash goes into the `git` key of the report and the audit
  line. `--dry-run` and runs stopped by lint or caps commit nothing. `git.commit` and `--no-commit`
  disable it. v1 never pulls and never pushes.

**Safety and hygiene**

- **R22.** Intake text is untrusted: fenced in prompts with a random per-run delimiter, delimiter
  look-alikes neutralized, never followed as instructions, never executed, slugs sanitized. slowhunch makes
  no network requests except to the configured provider `base_url`. Secrets come only from the env var
  named in config; never logged, never written to the vault, reports or state; redacted in errors. A note
  containing a secret-like pattern is never sent to a provider. `send_policy` default `excerpt_only`:
  only the intake text and the capped neighbour excerpts from `notes/` go to a provider; `inbox/` content
  other than the current intake is never sent.
- **R23.** Config caps: atoms per `asserts` input 5, per aesthetic input 3, characters per provider call
  12,000. A cap that bites is reported, never truncated silently.
- **R24.** Tests use temporary vaults only; with `SLOWHUNCH_TEST=1` a guard refuses any vault path inside
  a directory listed in `SLOWHUNCH_PROTECTED`.

## 3. Flow

1. Owner sends a link, a pasted poem, a file or a question to an agent.
2. For a link or search the agent reads it with its own tools; slowhunch is never asked to fetch.
3. Agent runs `slowhunch skill` once per session and `slowhunch verify`, then
   `slowhunch add <text|file|-> --as <medium> --title ... --author ... [--url ...] --note "why" [--hunch id]`
   (it asks the owner one short question about intent first if neither a note nor a hunch is available).
4. slowhunch normalizes the text, writes it to `inbox/`, hashes it, and returns the existing source on a
   hash match.
5. Gate -> extract (evidence and locator checks) -> place (TF-IDF neighbours, model verdict, code decides)
   -> write atoms and the source -> hunch match -> report JSON (contradictions first).
6. One git commit of exactly the written files.
7. Owner asks the book a question; the agent runs one `search`, follows `backlinks`, answers with `[[id]]`
   and exact quotes from the book only; nothing is stored unless the owner says so.

## 4. Tech stack

- Go 1.27 (installed: go1.27.3), `go.mod` module `github.com/0x6c6d/slowhunch`, go directive `1.27`.
- Standard library only: `flag`/`os`, `encoding/json`, `net/http` (provider calls only), `crypto/sha256`,
  `os/exec` (git and `exec` provider), `embed`, `testing`, `net/http/httptest`. No third-party package; no
  YAML library: frontmatter is a small, strict subset parsed and written by the project's own code (scalars,
  lists, list of maps for hunch evidence).
- Layout: `cmd/slowhunch/main.go`, packages under `internal/` (vault, frontmatter, intake, provider,
  pipeline, search, gitops, report, cli); tests beside the code (`*_test.go`).
- Tools: `git` 2.x on PATH, `make`, `gofmt`. No database, no web UI, no network service.
- `Makefile` targets: `build` (writes `./bin/slowhunch`), `test`, `lint` (`gofmt -l` must print nothing,
  plus `go vet ./...`), `check` (lint + test + build).
- License: MIT.

## 5. Constraints & out of scope

**Security (high-risk):**
- Secrets: env var names only, never values, never logged or stored; scrubbed environment for the `exec`
  provider; an `exec` provider command is run without a shell.
- Prompt injection: intake text is fenced and untrusted (R22); model output is never used as a path or a
  command; every write is confined to the vault with a symlink-resolved check.
- Outgoing network: only the provider `base_url` from config. No other host, ever.
- No force operations: slowhunch never runs `git push`, `git pull`, `--force` or a history rewrite in v1.

**Data / privacy (high-risk):**
- Only the current intake text and capped neighbour excerpts may be sent to a provider (`excerpt_only`).
- Store text and short quotes only; never raw video, images, audio or full copies of long works. Input
  over 12,000 characters is refused, which also bounds copyright exposure.
- Never store passwords, cookies, sessions or credentials.

**Legal (high-risk):** aesthetic text (poems, lyrics) is copyrighted material like any article; v1 stores
only what the owner sends, one work or passage per `add`, and the README states that the vault may hold
quoted excerpts, so a remote must be private.

**Performance (measurable):** `verify` and `search` on a generated vault of 2,000 atoms complete in under
5 seconds each on the dev machine; index rebuild is in memory with no on-disk cache required.

**Out of scope for v1 (must NOT be built):** web UI; any fetching of content by slowhunch (URLs, web
search, platform sites, feeds, ingest); an `anthropic` provider (dropped for good); `collide`, `review`;
git pull, push, `sync` and any remote handling; batch `add --from`; PDF extraction; the `docs/` folder
(README only) and `examples/`; MCP server; embeddings or a vector database; OCR, vision or audio models;
a daemon; multi-user sync; vault import; per-medium run-per-day caps; `send_policy` levels other than
`excerpt_only`.

**Files that must not be touched by workers:** the whole `.harness/` folder, `.claude/`, the "Delegation
workflow" section of `CLAUDE.md`, and the root `prompt.md` and `prompt_v2.md` (owner notes). `.harness/` is
managed by the loop.

## 6. Environment

- **Code location:** repo root `/home/sa/workspace/code/slowhunch`; no nested project folder, no `git init`.
- **Required tools and checks (each exits 0):**
  - Go 1.27: `go version | grep -q 'go1.27'`
  - git: `git --version`
  - make: `make --version`
  - gofmt: `gofmt -h 2>&1 | grep -q usage`
- **Secrets (names only):** `SLOWHUNCH_API_KEY` (default `api_key_env`; tests never need it). Other env
  vars: `SLOWHUNCH_VAULT` (vault path), `SLOWHUNCH_TEST`, `SLOWHUNCH_PROTECTED` (test guard).
- **Live systems the build may touch:** none. All provider tests use an in-process fake provider and
  `httptest` servers. No real LLM endpoint, no internet. Tests create vaults under `t.TempDir()` only.
  Everything else is off limits.

## 7. Deliverables

- Go source: `go.mod`, `cmd/slowhunch/`, `internal/...`, with tests; `Makefile`; `LICENSE` (MIT);
  `.gitignore` entry for `bin/`.
- Embedded default prompt files (gate, extract, place) with a `version` field.
- `README.md` at the repo root (written by the last task, not part of `.harness/`): what slowhunch is,
  install (`go install github.com/0x6c6d/slowhunch/cmd/slowhunch@latest` or `make build`), a short
  quickstart (init a vault, add one article text and one pasted poem, ask with `search`), the privacy note
  (private remote), usage of every command group, and configuration.
- **Config surface** (`.slowhunch/config.json`, written by `init`):
  - `provider`: `{name: "openai-compatible", base_url, model, api_key_env: "SLOWHUNCH_API_KEY"}`; for
    `exec`: `{name: "exec", command: [..]}`; optional per-role override under `roles`.
  - `caps`: `{atoms_per_asserts_input: 5, atoms_per_aesthetic_input: 3, chars_per_call: 12000}`.
  - `place`: `{near: 5, duplicate_cosine: 0.80, contradict_cosine: 0.20, echo_low: 0.20, echo_high: 0.80,
    excerpt_chars: 300}`.
  - `send_policy: "excerpt_only"`; `git: {commit: true}`.
  - Vault lookup order: `--vault <dir>`, `SLOWHUNCH_VAULT`, nearest parent directory containing
    `.slowhunch/`.
  - Global flags: `--json`, `--vault`, `--dry-run` (vault-changing commands), `--no-commit`.

## 8. Acceptance criteria

- [ ] `make check` exits 0 (gofmt clean, `go vet`, `go test ./...`, `go build ./...`). [R1-R24]
- [ ] Test: `init` on an empty temp dir creates all folders, config, prompts, identical `AGENTS.md` and
      `CLAUDE.md`, a `.gitignore`, and a first commit; a second `init` reports `EXISTS` for both agent
      files and leaves them byte-identical. [R1]
- [ ] Test: every file type round-trips through the frontmatter writer and parser, and `rev` is stable
      across a no-op write and changes after a field patch. [R2, R3, R5]
- [ ] Test: id generation matches the examples (`poem-larkin-trees-<4 hex>`,
      `crafts-larkin-trees-lines-1-2`) and collisions get `-2`. [R4]
- [ ] Test: a stale `--rev` update fails with `CONFLICT_REV` and returns current content; a path escaping
      the vault via `..` or a symlink is refused. [R5]
- [ ] Test: with the fake provider, `add` of a pasted Larkin couplet (`--as poem --title Trees --author
      "Philip Larkin" --hunch h1`) writes one source, a `crafts` and a `reads` atom with valid `locator`,
      an `inbox/` copy with numbered lines, hunch evidence, and one commit. [R7-R12, R21]
- [ ] Test: an atom whose `evidence` is not an exact substring of the stored text is dropped and counted in
      `errors`; a bad `locator` is dropped; `confidence` on a `crafts` atom is rejected. [R10, R16]
- [ ] Test: `.pdf` input exits `UNSUPPORTED`, 12,001 characters exits `TOO_LONG`, empty input exits
      `BAD_INPUT`, `--as film` without title exits `BAD_INPUT`, `--as art` without text exits
      `UNSUPPORTED` with the stated hint. [R7]
- [ ] Test: adding the same text twice writes nothing the second time; `--dry-run` leaves the vault
      byte-identical (hash of the tree before and after) and makes no commit. [R8, R12, R13]
- [ ] Test: gate provider error rejects the item and writes nothing; `--force` bypasses a gate reject.
      [R9]
- [ ] Test: place thresholds: a `duplicate` verdict at cosine 0.5 becomes `extends`; at 0.9 it appends a
      corroboration line and writes no new file; a `reads` atom at 0.95 creates a `rereads` link instead; an
      aesthetic `new` at cosine 0.4 yields an echo; a `contradicts` verdict with no shared tag and cosine
      0.1 becomes `extends`. [R11, R12]
- [ ] Test: a contradiction creates a flagged atom linked to both sides and a `weakens` hunch evidence
      entry, and the report lists it first; report keys appear in the specified order and every `[[id]]`
      in the report exists. [R12]
- [ ] Test: an agent sequence through `--json` succeeds without overwrite: create atom, link, hunch add,
      hunch evidence, soft delete (file now in `archive/`), lint exit 0. [R14, R17]
- [ ] Test: `hunch due` lists a hunch with a past `revisit` and omits others; `source facts` prints
      numbered lines. [R14]
- [ ] Test: `search` ranks a seeded 20-atom vault so the expected atom is first for a known query,
      honors `--medium`, `--stance` and `--budget`, and returns the `rereads` partner for a `reads` hit.
      [R15]
- [ ] Test: `lint` exits 3 for each of the four violations with a `--json` failure naming the item and
      writes nothing; `verify` on a hand-corrupted vault (altered evidence, dangling link, wrong rev, bad
      locator) reports each by `id` and code, changes nothing, exits 3. [R16]
- [ ] Test: `stats --json` on a seeded vault returns exact counts by `medium` and `stance` and the right
      `asserts`/`reads` ratio; `skill` output is at most 60 lines. [R16]
- [ ] Test: every command's `--json` output has `"schema": 1`; errors carry `code`, `message`, `hint`;
      exit codes 0/1/2/3/4 are produced by at least one test each. [R17]
- [ ] Test: `openai-compatible` against an `httptest` server sends the expected path, body fields and
      auth header, retries once on 429, and does one repair retry on invalid JSON; `exec` provider
      round-trips with a test script and gets a scrubbed environment (the test script prints its env and
      only `PATH` and the named key are present). No test uses a real network host. [R18-R20, R22]
- [ ] Test: the prompt text sent to the fake provider contains the intake fenced by a delimiter that does
      not occur in the text, and a text containing a look-alike delimiter is neutralized; a text with a
      secret-like pattern (`sk-...`) is never sent. [R22]
- [ ] Test: with a dirty foreign file in the vault, a run that would touch it exits 4 `CONFLICT` and
      writes nothing; a normal run commits exactly the written files (`git show --stat`) and never an
      unrelated modified file. [R21]
- [ ] Test: `git grep -E 'git (push|pull)|--force' -- '*.go'` outside test files returns nothing; the
      test guard refuses a protected path when `SLOWHUNCH_TEST=1`. [R21, R24]
- [ ] Test: a generated 2,000-atom vault runs `verify` and `search` in under 5 seconds each. [R15, R16]
- [ ] `README.md` exists, contains the sections install, quickstart, commands and configuration, and every
      command in its quickstart block runs in a test against a temp vault. [Deliverables]

## 9. Nice-to-haves

Not part of v1 (see `prompt_v2.md` for detail): `collide` and `review`; wildcard lane and topic append
flow; git pull, push, `sync`; batch `add --from`; PDF text extraction; full `docs/` set, `examples/vault/`
and scheduling docs; per-role provider tuning and run caps; further `send_policy` levels; better verse
search (index, embeddings); `prompts diff`; MCP wrapper; vault import note.
