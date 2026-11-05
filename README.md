# slowhunch

A commonplace book for AI agents: a plain Markdown vault, kept in git, that an agent writes to through a small CLI.

slowhunch stores atoms: one-line claims, each backed by a verbatim excerpt from the work it came from. Open questions are kept as hunches and collect dated evidence over time. When a new atom is placed in the vault, contradictions with existing atoms are surfaced first. Everything is plain Markdown under git, every command accepts `--json`, and slowhunch never fetches sources: you read the work yourself and pass the text in. It talks only to the configured model provider, an optional `pdf.command`, and the vault's own git remote when one is configured.

The idea is inspired by Steven Johnson's *Where Good Ideas Come From*. There is no evidence that these patterns actually improve ideas, which is why `slowhunch stats` exists: it reports counts, ratios and token usage so you can judge for yourself.

Features:

- Atoms with verbatim evidence, a stance (`asserts`, `crafts` or `reads`) and a locator.
- Hunches with a state (`open`, `warming`, `tested`, `dropped`) and supporting or weakening evidence.
- One-direction `[[id]]` links with computed backlinks and a link graph.
- Similarity search over notes.
- Revision numbers on every file, so conflicting updates are detected.
- Linting and verification of the vault against its files.
- Collide: proposals written from distant pairs of notes and hunches (`slowhunch collide`).
- Review of stale hunches, orphans and unplaced notes (`slowhunch review`).
- A wildcard lane that surfaces an occasional unexpected neighbour when placing atoms.
- Keep: link notes you chose to keep to a topic (`slowhunch keep`).
- Batch add: up to 50 files in one run and one commit (`slowhunch add --from`).
- PDF text through an external command (`pdf.command`).
- Per-role and per-medium providers, and per-medium caps on runs and tokens.
- Git sync: pull before write, opt-in push, and `slowhunch sync`.
- A built-in guide for agents (`slowhunch skill`).

## Requirements

- Go 1.27
- git 2.x
- make
- `pdftotext` (poppler), optional, only for `.pdf` input.
- A model provider: an OpenAI-compatible endpoint, or a local command (the `exec` provider).

## Installation

```sh
go install github.com/0x6c6d/slowhunch/cmd/slowhunch@latest
```

From source:

```sh
git clone https://github.com/0x6c6d/slowhunch
cd slowhunch
make build
```

The binary is written to `./bin/slowhunch`.

## Quick start

A model provider must be configured before `slowhunch add` can work; see [Configuration](#configuration).

```sh
slowhunch init commonplace
cd commonplace
slowhunch add - --as article --title "Notes on slow reading" --author "A. Reader" --note "a small test of the intake" <<'EOF'
Slow reading is the habit of returning to a passage until it gives up more than the first pass did. A reader who copies out a sentence by hand notices its rhythm, its word order and what it leaves unsaid.
EOF
slowhunch add - --as poem --title "Trees" --author "Philip Larkin" --note "an image of something almost said" <<'EOF'
The trees are coming into leaf
Like something almost being said;
EOF
slowhunch search "trees leaf"
slowhunch verify
```

## Usage

Run `slowhunch help` for the command list. All commands take the global flags described below.

### init

Create a vault (a git repository with a config, prompts and folders) in a directory.

```sh
slowhunch init commonplace
```

### add

Ingest one item: a piece of text, a file, or `-` for standard input. slowhunch gates it, extracts atoms with verbatim evidence and places them among existing notes. `add` always prints JSON.

```sh
slowhunch add notes/essay.txt --as article --title "On slow reading" --author "A. Reader" --url "https://example.com/essay" --note "why I saved it" --hunch does-slow-reading-help
```

Flags:

- `--as` the medium, for example `article`, `paper`, `blog`, `post`, `thread`, `page`, `code`, `book`, `talk`, `poem`, `lyric`, `prose`, `quote`, `film`, `art`, `own` or `other`. It is inferred when omitted.
- `--url` where the work came from (recorded only, never fetched).
- `--title`, `--author` descriptive metadata. A title is required for `poem`, `lyric` and `film`.
- `--note` why the item matters to you.
- `--hunch` a hunch id the item bears on.
- `-force` add the item even if it was already added.
- `--dry-run` (global) show what would happen without writing anything.

```sh
slowhunch add - --as quote --title "Letter" --dry-run <<'EOF'
Chance favors the connected mind.
EOF
```

### add --from

Ingest several files in one run. The batch file has one line per item, either `file` or `file|url|title|author`. A batch holds at most 50 entries and produces one commit.

```sh
slowhunch add --from batch.txt --as article --note "weekend reading"
```

```text
notes/essay.txt
notes/post.txt|https://example.com/post|On habits|A. Reader
```

With `--from`, `--url`, `--title` and `--author` are not allowed on the command line; put them on the batch line. `--as`, `--note`, `--hunch` and `-force` apply to every line.

### PDF input

`add` accepts a `.pdf` file when `pdf.command` is set (the default runs `pdftotext -layout FILE -`, which needs poppler installed). The extracted text goes through the same pipeline as any other input.

```sh
slowhunch add papers/study.pdf --as paper --title "A study" --author "A. Reader"
```

### collide

Pair notes and hunches that are far apart in similarity, ask the model for a proposal for each pair and write the kept ones as proposal entries linked to both items.

```sh
slowhunch collide --limit 3
slowhunch collide --dry-run
```

`--limit` caps the pairs for this run (it must be at least 1). Pair selection is tuned by `collide.max_cosine`, `collide.aesthetic_weight` and `collide.max_pairs`.

### review

List stale hunches, orphan notes and unplaced notes. Read-only.

```sh
slowhunch review --stale-days 30
```

`--stale-days` overrides `review.stale_days`.

### keep

Keep a piece of your own text and link it to the notes it came from, under a topic.

```sh
slowhunch keep "Copying by hand slows me down enough to notice." --ids crafts-larkin-trees-lines-1-2 --topic trees
```

`--ids` is a comma-separated list of entry ids. Without `--topic`, the nearest topic is used.

### sync

Pull and then push the vault's git remote. It writes no vault file and makes no commit. See [Git sync](#git-sync).

```sh
slowhunch sync
slowhunch sync --no-push
```

### create, get, update, delete, list

Create an entry from JSON (types: note, source, hunch, proposal, topic), read it back, patch it with its current `--rev`, archive it, or list entries.

```sh
slowhunch create topic --data '{"title": "Trees"}' --id trees
slowhunch get crafts-larkin-trees-lines-1-2
slowhunch update trees --rev 000001 --data '{"tags": ["poetry"]}'
slowhunch update trees --rev 000002 --append "Notes on leaves and seasons."
slowhunch delete trees
slowhunch delete trees --hard --yes
slowhunch list --type note --medium poem --limit 20
```

`delete` archives an entry; with `--hard --yes` it removes the file. `list` also accepts `--stance`, `--tag`, `--hunch` and `--state`.

### link, unlink, backlinks, graph

Links go one way, from one entry to another. Backlinks are computed.

```sh
slowhunch link crafts-larkin-trees-lines-1-2 trees
slowhunch unlink crafts-larkin-trees-lines-1-2 trees
slowhunch backlinks trees
slowhunch graph trees --depth 2
```

### hunch

A hunch is an open question. Add one, attach a note as evidence, move it through its states, and list the hunches due for revisit.

```sh
slowhunch hunch add "Does copying by hand improve recall?" --next-test "copy five passages and recall them a week later" --revisit 2026-12-01 --tags memory,reading
slowhunch hunch evidence does-copying-by-hand-improve-recall --note crafts-larkin-trees-lines-1-2 --relation supports
slowhunch hunch state does-copying-by-hand-improve-recall warming
slowhunch hunch due
```

Relations are `supports` and `weakens`. States are `open`, `warming`, `tested` and `dropped`.

### source facts

Print the stored text of a source, numbered by line, so evidence can be quoted exactly.

```sh
slowhunch source facts poem-philip-larkin-trees
```

### search

Search notes by similarity.

```sh
slowhunch search "trees leaf" --limit 5 --json
```

Flags: `--medium`, `--stance`, `--tag`, `--hunch` (restrict to notes near a hunch), `--budget` (token budget, 0 is unlimited) and `--limit` (default 10).

### lint, verify, stats, skill

```sh
slowhunch lint
slowhunch verify
slowhunch stats
slowhunch skill
```

`lint` checks the vault against the note rules. `verify` re-derives the index from the files and reports inconsistencies; it is read-only. `stats` shows counts, ratios and token usage; it is read-only. `skill` prints the guide an AI agent should read at the start of a session.

### Global flags

- `--json` print a JSON envelope instead of text.
- `--vault` use this vault directory (otherwise `SLOWHUNCH_VAULT`, otherwise the nearest parent directory containing `.slowhunch`).
- `--dry-run` do not write anything.
- `--no-commit` write files but do not create a git commit.
- `--no-pull` skip the pull before a vault-changing command.
- `--push` push after the run's commit, even when `git.push` is false.
- `--no-push` never push (cannot be combined with `--push`).
- `--sync` pull first for read-only commands too.

```sh
slowhunch --vault ~/commonplace --json get crafts-larkin-trees-lines-1-2
```

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success. |
| 1 | Fatal error: configuration, provider failure, missing git, or another internal error. |
| 2 | Bad input: unknown command, wrong arguments, entry not found, input too long, unsupported option. |
| 3 | The vault failed lint. |
| 4 | Conflict: stale `--rev`, duplicate content or an entry that already exists. |

Error codes `CAP_REACHED` (a configured cap stopped the run) exits 2. `PULL_FAILED`, `PUSH_FAILED` and `BEHIND` exit 1; `BEHIND` is reported as a warning when the vault is behind its upstream.

### JSON output

With `--json`, success prints an envelope with `"schema": 1`, `"ok": true` and the command's fields:

```json
{
  "schema": 1,
  "ok": true,
  "problems": [],
  "checked": 12
}
```

Errors use `"ok": false` and an `error` object with `code`, `message` and `hint`:

```json
{
  "schema": 1,
  "ok": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "no entry with id missing-entry",
    "hint": "try slowhunch search"
  }
}
```

Reports written by `add` and `collide` also carry the keys `wildcards` (items surfaced by the wildcard lane), `collisions` (kept collide pairs with their proposal) and `git` with `pulled`, `pushed` and `behind`.

## Configuration

Each vault has a `.slowhunch/config.json`, written by `slowhunch init`. Values you omit keep their defaults.

| Name | Default | Effect |
| --- | --- | --- |
| `provider.name` | `openai-compatible` | Provider kind: `openai-compatible` or `exec`. |
| `provider.base_url` | `http://localhost:11434/v1` | Endpoint of the OpenAI-compatible API. The only host slowhunch contacts. |
| `provider.model` | `llama3.1` | Model name sent to the endpoint. |
| `provider.api_key_env` | `SLOWHUNCH_API_KEY` | Name of the environment variable that holds the API key. |
| `provider.command` | none | For `exec`: the program and arguments to run (no shell). |
| `roles` | none | Per-role overrides of the provider fields for `gate`, `extract`, `place` and `collide`. Precedence: `media.<medium>.<role>` over `roles.<role>` over `provider`. |
| `media` | none | Per-medium, per-role provider overrides, for example `media.poem.extract`. |
| `caps.atoms_per_asserts_input` | 5 | Maximum atoms taken from one factual input. |
| `caps.atoms_per_aesthetic_input` | 3 | Maximum atoms taken from one poem, lyric, prose or similar input. |
| `caps.chars_per_call` | 12000 | Maximum characters per model call and per input. |
| `place.near` | 5 | How many nearby notes are considered when placing an atom. |
| `place.duplicate_cosine` | 0.80 | Similarity at or above which an atom counts as a duplicate. |
| `place.contradict_cosine` | 0.20 | Similarity threshold used for contradiction candidates. |
| `place.echo_low` | 0.20 | Lower bound of the similarity band for echoes. |
| `place.echo_high` | 0.80 | Upper bound of the similarity band for echoes. |
| `place.excerpt_chars` | 300 | Length of the excerpts sent to the model. |
| `place.wildcard_share` | 0.10 | Share of placement slots given to the wildcard lane. |
| `collide.max_cosine` | 0.15 | Pairs must have similarity at or below this value to collide. |
| `collide.aesthetic_weight` | 2.0 | Weight favouring aesthetic notes when ranking pairs. |
| `collide.max_pairs` | 6 | Maximum pairs per collide run. |
| `review.stale_days` | 60 | Days without evidence after which a hunch is stale. |
| `pdf.command` | `["pdftotext", "-layout", "{file}", "-"]` | Program and arguments that print the text of a PDF; `{file}` is replaced by the path. |
| `caps.by_medium.<medium>.runs_per_day` | 0 | Maximum runs per day for the medium (0 is unlimited). |
| `caps.by_medium.<medium>.tokens_per_run` | 0 | Maximum tokens per run for the medium (0 is unlimited). |
| `send_policy` | `excerpt_only` | What is sent to the model. Only `excerpt_only` is supported. |
| `git.commit` | `true` | Commit every change to git. |
| `git.pull` | `true` | Pull (rebase) before a vault-changing command when a remote and upstream exist. |
| `git.require_pull` | `false` | Fail with `PULL_FAILED` instead of warning when the pull fails. |
| `git.push` | `false` | Push after each commit. |
| `git.remote` | `origin` | Remote used for pull and push. |
| `git.branch` | current branch | Branch pushed to the remote. |

Environment variables:

- `SLOWHUNCH_API_KEY` is the default API key variable (see `provider.api_key_env`).
- `SLOWHUNCH_VAULT` is the vault directory used when `--vault` is not given.

The vault is found in this order: the `--vault` flag, the `SLOWHUNCH_VAULT` variable, then the nearest directory from the current one upwards that contains `.slowhunch`.

A configuration that runs a local command as the model. The command receives a JSON request on standard input and writes a JSON reply to standard output; its environment is limited to `PATH`.

```json
{
  "provider": {
    "name": "exec",
    "command": ["/usr/local/bin/my-model", "--stdio"]
  }
}
```

An example with a per-medium provider override, a per-medium cap and push enabled:

```json
{
  "media": {
    "poem": {
      "extract": {"name": "openai-compatible", "model": "llama3.1:70b"}
    }
  },
  "caps": {
    "by_medium": {
      "article": {"runs_per_day": 20, "tokens_per_run": 20000}
    }
  },
  "git": {"push": true}
}
```

## Git sync

Before a command changes the vault, slowhunch runs `git pull --rebase --autostash` when the vault has the configured remote and an upstream branch. It never creates a merge commit. A rebase conflict fails with `CONFLICT` and nothing is written. Pushing is opt-in: set `git.push` or pass `--push`. `slowhunch sync` pulls and pushes on demand. slowhunch never force-pushes and never rewrites history. Keep any remote private, since the vault holds excerpts of other people's work.

## Privacy and copyright

A vault stores quoted excerpts of other people's work. Keep any git remote you add to it private.

slowhunch contacts only the configured `base_url` (or runs the configured local command), runs `pdf.command` for PDF input, and uses the vault's git remote when one is configured. It sends excerpts, not whole vaults. Text that looks like a secret is refused, and inputs over 12,000 characters are refused.

## Project structure

```text
cmd/slowhunch/        program entry point
internal/cli/         command dispatch, flags and output envelopes
internal/errs/        error codes and exit codes
internal/frontmatter/ Markdown front matter reading and writing
internal/gitops/      git commits and guarantees
internal/intake/      input reading, normalisation and secret checks
internal/pipeline/    add pipeline: gate, extract, place, write
internal/policy/      repository policy tests
internal/provider/    model providers (openai-compatible, exec)
internal/report/      reports and statistics
internal/search/      similarity search
internal/synth/       collision pair selection
internal/vault/       vault layout, config, index and transactions
```

## Development

```sh
make build   # build ./bin/slowhunch
make test    # go test ./...
make lint    # gofmt check and go vet
make check   # lint, test and build
```

## License

MIT, see `LICENSE`.
