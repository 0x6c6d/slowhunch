# slowhunch: project description

> Build brief for a new, standalone open-source project. Decisions marked **(default)** are
> proposals where the owner was silent and can be overruled. *(inferred)* marks anything that was
> not stated by the owner or read in a source.

## One-line goal

**slowhunch** is a Go program plus a vault format that turns a personal knowledge base into a
**commonplace book optimized for AI agents**. You give it material — text, files and
anything an agent reads for you (pages, search results), and the written forms of what you find beautiful: poems, songs, novels, sacred and
philosophical texts, next to articles, papers and blog posts — agents read and analyze it, store it as
small linked claims and excerpts, attach those to long-running **hunches** (open questions that collect
evidence over months), and surface contradictions and new combinations. Agents can create, read, update
and delete everything through a CLI with JSON output.

The book has **no human-written prose layer and no UI**. The owner only feeds material, opens questions
and asks the agent; everything inside is machine-addressable: typed frontmatter, verbatim excerpts,
links by `[[id]]`. Understanding is assembled by the agent at question time, out of those pieces, and
lives in the conversation; the book itself stores no argument. That is also why poetry and art belong
here rather than in a separate library folder: they are the far material combinations are made of, and
a vault of only articles has nothing distant to combine.

## Why this exists

A normal notes folder stores what you read. A commonplace book stores what survives out of what you
read — the line, the argument, the picture, the fragment of philosophy — and lets connections between
them grow slowly. It works on articles, papers and code repos; it works the same on poems, lyrics,
novels and paintings. The design follows Steven Johnson, *Where Good Ideas Come From* (2010), seven
patterns *(inferred from the book as remembered; a web search of the Penguin catalogue page confirmed
only the first five names)*:

| Pattern | How it shows up in slowhunch |
|---|---|
| Adjacent possible | every atom records what it **unlocks**: what became buildable, testable or readable-next |
| Liquid networks | new material is compared with **near and far** neighbours, plus a wildcard lane; poems and papers sit in the same graph on purpose |
| Slow hunch | **hunches** are open questions with a dated evidence log and a revisit date |
| Serendipity | near-miss rejections are listed as **wildcards**, never dropped silently |
| Error | **contradictions** between atoms are the most valuable output and always reported first |
| Exaptation | every atom records where it could be **repurposed**; art and philosophy are the far material, and both earn their place for that reason alone |
| Platforms | stable file format, JSON CLI so any agent can build on the book |

There is no evidence that these patterns improve idea quality. Treat slowhunch as an experiment and
build the measurement in (`slowhunch stats`).

## Core concepts

Every stored thing is one of five typed files. All of them are claims-shaped and machine-readable; none
is prose the owner has to keep up.

- **Atom (note):** one statement in one file, with a **verbatim** `evidence` excerpt and a link to its
  source. `stance` says which kind it is:
  - `asserts` — a claim, method, number, case or argument (papers, articles, philosophy). Has
    `confidence`, and philosophical claims start low.
  - `crafts` — a technique or observed effect ("the caesura before the last word does the work"). No
    confidence.
  - `reads` — an interpretation of a text, image or argument by the agent. No confidence, always
    revisable. On a philosophical passage the author's argument is an `asserts`, your reading is a
    `reads`.
- **Source:** thin provenance record for whatever was analyzed: article, paper, video, post, page,
  poem, lyric, novel, painting. Holds the excerpt and the citation, never a full copy.
- **Hunch:** an open question with state (`open`, `warming`, `tested`, `dropped`), a next test, and
  dated evidence entries that support or weaken it.
- **Proposal (collision):** an idea made by combining two or three distant atoms or hunches, with the
  cheapest test. It stays a proposal until the owner promotes it.
- **Topic:** synthesis page, one per recurring question the owner actually asks. Agents may only
  append, only when the owner says to keep an answer, and only as a dated, quoted, linked block, never a
  fresh prose file.

A poem yields one `crafts` atom and one `reads` atom, not a summary. A paper yields up to five
`asserts` atoms. A philosophical passage yields `asserts` for its argument and `reads` for what the
argument does. All are just atoms: nothing downstream treats them differently except the weights in
`collide` and the gate question.

A `worldview` is not a file type. It shows up at three surfaces and nowhere else: `bears_on` (what an
atom serves), `proposals/` (combinations offered to the owner), and an appended `topics/` block when the
owner keeps an answer. Everything else stays retrieval.

## Inputs: what the owner can give it

The owner hands slowhunch **anything they want analyzed**, as **text**. slowhunch itself never touches
the network for content: it has no fetchers. When the owner sends a link or asks for a web search, the
**agent** reads the page or runs the search with its own tools and passes the resulting text, with the
URL as citation, to `add`. It makes no difference between "knowledge" and "taste".

1. **Text** on stdin or as an argument (`add -`): an article body, verse, lyrics, a quotation, a
   transcript, an owner's description of a painting or scene. This is the normal path for everything,
   and the only one for written beauty, which rarely has a stable public URL.
2. **A text/Markdown/PDF file** (PDF text extraction is optional, see Out of scope).
3. **Batch files**: a list of files and notes, e.g. `slowhunch add --from batch.txt`, one entry per line
   (`<file>` or `<file>|<url>|<title>|<author>`).

The `--url` flag records where the agent got the text; it is a citation only and is never requested.

One command covers it: `slowhunch add <text|file|-> [--as <medium>] [--url "..."] [--title "..."]
[--author "..."] [--note "why I care"] [--hunch <id>]`. `--as` takes any `medium` value, sets it, and
picks the gate and extract variant; without it the medium is inferred from file extension and length,
and an agent should say which one it means. `--title` and
`--author` are how pasted verse gets a citation and a stable slug; `--as poem|lyric|film`
requires `--title`, `--as quote` requires `--author`, and unknown authorship is written `anonymous`, never guessed. `--note` and `--hunch`
carry the owner's intent into the analysis and are the strongest signal a gate and an extract prompt
get, so an agent always asks for one or derives it from the owner's message before calling.

**What enters from material that has no text.** A poem, a lyric, a painting or a quote from a book is
added as text with `--as`. The owner sends it once; the agent quotes it back exactly, and the `inbox/`
copy is the ground truth for the substring check. Images and audio are **never stored as files** —
the owner's description of what is in the picture is pasted as text and becomes the evidence; the
`media` field carries a link only. Full works are not mirrored: one poem yes, a whole novel is added as
the passages the owner sends. `--as art` or `--as film` without any pasted text fails with `UNSUPPORTED` and the hint
"describe or quote the image or scene; there is no vision or audio model". A film or a scene enters the
same way a painting does: the owner's description, or the quoted dialogue or screenplay lines, is the
evidence.

**Intake rules that keep an agent honest.** The owner's `--note` is stored verbatim as `owner_note` on
the source, not only fed to the prompt: it is the cheapest retrieval key later and the only trace of why
a thing was kept. The normalized intake text is hashed; the same hash returns the existing source instead
of writing a second one, and only the atom layer decides whether that is a `rereads`, a `crafts` variant
or nothing. Slugs for aesthetic sources are `poem-<author>-<title>-<short>` (and the same for lyric,
prose, quote, film, art; `own-<title>-<short>` for `own`), because translations and versions of one work would otherwise collide on `id`.

### Intake normalization **(default)**

- **No fetchers, no network for content.** slowhunch takes text, a file or stdin and nothing else.
  There is no fetcher interface, registry, web backend, platform support or plugin. URLs, searches,
  transcripts and social posts are the agent's job: it reads them with its own tools and pipes the text
  in with `--url` as citation. Anything the agent could not read, the owner pastes.
- A **`text`** reader (stdlib) turns stdin, an argument or a file into a normalized `Document`.
- A **`verse`** pre-step (stdlib, code, no model) runs for `--as poem|lyric|prose|quote|film`: it normalizes
  whitespace, keeps line breaks, and numbers every line, so a `locator` is a stable address and the
  substring check is unambiguous even after the agent's prompt round-trip. Curly quotes, em dashes and
  non-breaking spaces are normalized once, at intake, and the normalized text is what `inbox/` keeps
  and what evidence is compared against.
- For `--as code` the same step only numbers the lines and leaves whitespace untouched, because
  indentation is part of the code.
- Normalized `Document`: `url` (citation, optional), `medium`, `title`, `author`, `published`, `text`,
  `locator` base (line numbers), `media` (links only), `retrieved_at`, `warnings`. Errors use the codes
  `BAD_INPUT`, `UNSUPPORTED` and `TOO_LONG`.
- Store **text and short quotes only**, never raw video, images or full copies of pages in the vault;
  keep the original URL as the citation.
- slowhunch never logs in, stores passwords, cookies or sessions, or writes to a remote site.

## How analysis works

The pipeline is identical for every medium; only the gate question, the atom count and one field change.
A poem and a paper are read, checked, placed and reported by the same code path, and a single
`collide` can pair one with the other. The rules below are code rules wherever they can be: an agent
that forgets them gets an error, not a bad file.

1. **Read:** normalize the input text into a `Document`; `--as` overrides the inferred `medium`.
2. **Gate (agent):** cheap, fails closed on error. Exactly one question is asked, chosen by `medium`:
   - **factual** (`paper`, `article`, `blog`, `page`, `post`, `thread`, `code`, `talk`, `book`): is there a
     concrete claim, method, number, case or technique, or an argument worth following? In a `thread`
     the different voices and their disagreements are the content; in `code` it is what the code
     does or how it is built, never a summary of the repository. A philosophical or sacred passage is
     `--as book` and comes here: it argues rather than illustrates.
   - **aesthetic** (`poem`, `lyric`, `prose`, `quote`, `film`, `art`): is there a line, image or passage that works, and
     a hunch or neighbouring atom it touches? Aesthetic material passes on the excerpt alone and does
     not need a thesis. A beautiful passage from a novel is `prose`; an argument from the same novel is
     `book`. A `quote` is a short stand-alone saying (aphorism, proverb, epigraph); one that makes an
     argument rather than a point is `--as book`.
   - **owner-steered** (`other`, `own`, or any medium with a `--note` or `--hunch`): keep it because the owner
     named it, even if the gate above would fail.
   An explicit owner `add` can bypass the gate (`--force`).
3. **Extract (agent):** `asserts` material 1 to 5 atoms, aesthetic material 1 to 3, with `claim`,
   `evidence`, `stance`, `tags`, `unlocks`, `repurpose`, and `confidence` only when `stance=asserts`.
   The rules, all enforced in code and never left to the prompt:
   - `evidence` must be an exact substring of the stored text, or the atom is dropped and counted. This
     blocks hallucinated claims and misquoted verse.
   - `locator` is required when `medium` is aesthetic: a line or stanza range, song section, verse,
     timecode, or `region:x1,y1,x2,y2` for an image. It is also required for `thread` (`comment N`, so
     each voice is addressable) and `code` (`lines N-M`).
   - **`art` and `film`:** the `evidence` is the owner's description (or the quoted dialogue or
     screenplay lines), quoted back verbatim. The description, not
     the picture, is what gets checked and what future agents read.
   - **`own`:** the owner's own observation, dream or idea, pasted as text. The source `author` is
     the owner (never `anonymous`), authorship is `human`, and there is no copyright concern. It
     yields `asserts` atoms with low `confidence` by default, or `reads` when it interprets something.
   - **Foreign-language text:** `evidence` stays in the original language, the English gloss goes in
     `rendering`, and only `evidence` is checked. Never translate inside `evidence`, never gloss a poem
     line without quoting the line.
   - **Aesthetic material:** never a paraphrase and never a summary. If the atom could stand in for the
     poem, it is too long; cut it to the line that caused the reaction and let the `claim` carry the
     reaction.
4. **Place (agent):** compare with near neighbours (TF-IDF) and one far neighbour; verdict
   `duplicate | new | extends | contradicts`. **Code decides** what is written from the verdict and
   thresholds, not the model. `reads` atoms are never dissolved into a duplicate even on high
   similarity: a second reading of the same line is a new atom linked with `rereads` to the first,
   because the difference between two readings is the content and the pair is a contradiction candidate
   rather than noise. Aesthetic material rarely reaches `contradicts` honestly, so a near-miss score on
   it becomes an **echo candidate** instead of being dropped by the threshold; echoes are placed, not
   filed as wildcards.
5. **Write:** new atom + thin source page; duplicate = dated corroboration line appended (never for
   `reads`); contradiction = flagged atom linked to both sides and hunch evidence. A translation is
   placed against its original with `translates` on the source and a normal `extends` or `contradicts`
   verdict on the atoms: two translations of one line are the cheapest contradiction generator there is
   and never a duplicate.
6. **Hunch match:** append dated evidence to open hunches the atom bears on (and to the hunch named
   with `--hunch`).
7. **Report:** what was added, in this order: contradictions, hunches that moved, new atoms,
   corroborations, **echoes** (an aesthetic atom placed beside an existing atom without contradicting
   it), wildcards, collisions, counts and errors, token cost. Echoes are the aesthetic counterpart of
   corroboration and exist so a poem-only day does not report as empty.

Scheduled: `slowhunch collide`
(weekly: combine distant items into a proposal, aesthetic atoms weighted up; a fixed share of every run
is spent on `asserts` × `reads` across media, the lane where a poem can move a paper), `slowhunch
review` (monthly: stale hunches, orphans, unplaced aesthetic atoms).

## Inference providers (open, no vendor lock-in)

- One Go interface `Provider` with a registry; config selects provider, `base_url`, `model` and
  `api_key_env`. Roles `gate`, `extract`, `place`, `collide` can use different providers and models.
  Gate and extract take a `medium` variant, so a cheap model can handle page posts while a stronger one
  does verse.
- Built-in **(default)**: `openai-compatible` (OpenAI, OpenRouter, Ollama, vLLM, LM Studio, and any
  other `/v1/chat/completions` service), `anthropic` (Messages API), and `exec` (a user command with
  JSON on stdin and stdout, so a new provider needs no Go code).
- Structured output is requested as JSON, validated in Go, retried once with a repair prompt, then
  counted as a per-item error. Model output is never trusted as instructions or file paths.
- No provider is special. There is no scoring service in the decision path: all decisions come from
  LLM agents plus deterministic code checks.
- **Cost shape:** gate is one short call per item, extract one call per item, place one call per new atom
  (never batched, it needs live neighbours). Aesthetic adds are the cheapest path in the system and are
  the right default when the owner is unsure; `collide` is the most expensive and is capped in config.
  Every report line ends with tokens spent per role so the owner can see what a poem costs against what a
  paper costs and set caps by `medium`.

## Vault format

Plain Markdown with frontmatter in a git repo, readable in Obsidian, no plugin syntax. A vault is
created with `slowhunch init <dir>` and is **separate from the program repo**.

```
notes/       atoms: one claim, craft observation or reading per file
hunches/     open questions with evidence logs
sources/     provenance for each analyzed input
topics/      synthesis pages (append-only for agents)
proposals/   collisions waiting for promotion
inbox/       raw intake text, immutable once written
archive/     soft-deleted items
.slowhunch/  config.json, state.json, prompts/, reports/, audit.log, cache/
AGENTS.md    bootstrap for any agent started in the vault (written by `init`)
CLAUDE.md    same text as AGENTS.md, for tools that read that name instead
```

`init` writes `AGENTS.md` and `CLAUDE.md` and nothing tool-specific beyond them: no `.claude/skills/`
or other skill folders. Both files hold the same short text (a real file each, not a symlink): what this
folder is, "run `slowhunch skill` at session start and follow it", "run `slowhunch verify` before and
after any batch", and the rule that the agent writes only through the `slowhunch` CLI. The rules
themselves live only in `slowhunch skill`, so there is one copy to keep current. `init` never overwrites
an existing `AGENTS.md` or `CLAUDE.md`; it reports `EXISTS` for the file and leaves it alone.

No folder is prose. Every file is typed, frontmatter-first and addressable by `id`, so an agent can act
on it without parsing an essay.

Frontmatter on all types: `id` (unique kebab-case slug = filename), `type`, `created`, `updated`
(absolute dates), `author` (`human` or `agent:<provider>/<model>`), `rev` (content hash), `tags`, plus
`bears_on` (hunch or proposal ids the item serves, so an atom can point up without becoming prose).
Type-specific: note (`claim`, `evidence`, `stance`, `locator` when aesthetic, `source`, `status`,
`unlocks`, `repurpose`, `confidence` only for `asserts`, `rereads` for a repeat of an existing reading,
`rendering` for a gloss, `translates` for the same line in another language), hunch (`question`,
`state`, `next_test`, `evidence` list of `{date, note, relation}`, `revisit`), source (`url`, `medium`,
`title`, `author`, `published`, `atoms` taken, `translates` when one work exists
in two languages, `owner_note`), proposal (`items`, `idea`, `cheapest_test`, `status`). Links are
`[[id]]`, one direction only, backlinks computed. Statements an agent inferred rather than read are
marked *(inferred)*.

`medium` is `paper | article | blog | post | thread | page | code | book | own | poem | lyric | prose |
quote | film | art | talk | other`.
It is a routing and weighting field, not a taxonomy to maintain. `stance` is `asserts | crafts | reads`.
A philosophical or sacred text is `book` and takes the factual gate; it is not a separate medium,
because what differs is the argument's `confidence`, not its handling. Its `confidence` stays low by
default: a philosophical claim is not corroborated by a second philosopher quoting it.

`locator` is a closed set of shapes so code can validate it against the stored excerpt:
`line N`, `lines N-M`, `stanza N`, `verse N`, `comment N`, `section <name>`, `timecode 1:23-1:31`,
`region x1,y1,x2,y2`. A locator that does not match a real line, stanza or timestamp in the source is a
lint failure, which is what makes "where exactly" checkable rather than a claim about a claim.

The atom body has exactly two parts and no more: the `claim` as one line, then the excerpt as a
blockquote. Never prose, never a second paragraph. The `locator` lives in the frontmatter, where code can
check it. `evidence` is the blockquote text with the `> ` stripped, so the substring check runs against
the stored excerpt and not against the rendered file.

```markdown
---
id: crafts-larkin-trees-enjambment
type: note
stance: crafts
source: poem-larkin-trees
locator: lines 1-2
unlocks: ["checking whether the flat enjambment is what keeps the simile from being cute"]
tags: [diction, enjambment]
rev: 3f9a1c
---

The line break holds the simile flat instead of letting it become decorative.

> The trees are coming into leaf
> Like something almost being said
```

Every atom names the `source` it came from and the line it quotes; a rewrite changes a field and bumps
`rev`, never the excerpt.

`lint` enforces the four things an agent forgets: an aesthetic atom without `locator`, a `confidence` on
a non-`asserts` atom, a `rendering` where `evidence` belongs, and a `reads` atom with no `rereads` link
to the reading it repeats. Exit code 3 and a `--json` failure per item; nothing is written for a linting
atom.

## Management, determinism and hygiene

The book is only worth something if a future agent, with no memory of today, can act on it correctly. So:

- **Prompts are data.** The gate, extract and place prompts live in `.slowhunch/prompts/` as files with
  a `version` field, committed with the vault. Their version is recorded in `state.json` and in the
  audit line of every run, so a change in output quality is traceable to a change in prompt.
  `slowhunch prompts diff <n>` shows what an upgrade would change; an agent reports a prompt change in
  the same breath as the atoms it produced.
- **Idempotent by default.** Running the same `add` twice writes nothing new (source hash, then the
  atom-level verdict). `--dry-run` is byte-identical and prints the plan. A retried run after a crash
  resumes from the last written file, never from the start.
- **Bounded writes.** Config caps atoms per input and per run, runs per day per `medium`, and
  characters per provider call. An agent that hits a cap reports it; it does not silently truncate a poem
  mid-stanza.
- **Stable ids, immutable bodies.** Ids derive from type, source and locator, not from the claim text,
  so re-wording a claim never renames a file or breaks a link. Bodies are append-only; corrections are a
  new atom or a field patch, never an in-place rewrite of an excerpt.
- **One machine-readable report per run** in `.slowhunch/reports/`, plus stdout. Both carry the same JSON.
- **Every failure is a code, never a sentence.** An agent's retry logic keys on `code`; `hint` is for
  the model's eyes and is short. A dropped atom is reported with its reason and its source, never
  silently skipped.
- **The vault is the state.** No hidden database, nothing to rebuild by hand, `git status` is the truth
  of what an agent changed. `slowhunch verify` re-derives the index from files, checks every `evidence`
  substring, every `locator` range and every link, and is safe before and after any batch.
- **Every change is committed, and pushed when there is a remote.** Any command that changes the vault
  (`add`, `collide`, `review`, `create`, `update`, `delete`, `link`, `unlink`, `hunch ...`,
  `init`) ends with one git commit per run, never one per file. The rules:
  - The commit contains exactly the files that run wrote or moved, never `git add -A`, so a manual edit
    the owner has not committed is not swept in. If a run would touch a file that has uncommitted
    changes it did not make, it stops with `CONFLICT` (exit 4) before writing.
  - The message is machine-shaped and greppable: `slowhunch: <command> <ids> (<counts>)`, with the run
    id and prompt versions in the body; the author is `slowhunch` or the `agent:<provider>/<model>` that
    ran it. The commit hash goes into the `audit.log` line and the report (`git` key).
  - `--dry-run` and a run that fails lint or is stopped by a cap commit nothing. A crash mid-run leaves
    its files uncommitted, and the resumed run commits them together.
  - If the vault has a remote (default `origin`, and the current branch has an upstream), the commit is
    pushed right after it with a plain `git push`: never `--force`, never a history rewrite. A push that
    fails (offline, auth, non-fast-forward) never undoes or blocks the write: the commit stays local,
    the report carries `PUSH_FAILED` with a short hint, and the next successful run pushes everything
    pending. With no remote, it only commits.
  - `init` runs `git init` when the directory is not yet a repo, writes a `.gitignore` for
    `.slowhunch/cache/`, and makes the first commit. If `git` is not installed, writes still succeed and
    the report carries `GIT_MISSING`.
  - Config under `git`: `commit` (default true), `push` (default true, only acts when a remote exists),
    `remote`, `branch`. `--no-commit` and `--no-push` override per run. slowhunch stores no git
    credentials: push uses the owner's own ssh agent or credential helper.
- **Pull before every write, so several copies of the vault stay clean.** The owner may keep the vault
  on several machines. Before the first write of any vault-changing run, when a remote exists,
  slowhunch syncs with it first:
  - It runs `git pull --rebase --autostash`, so commits that could not be pushed earlier are replayed on
    top of what arrived, and uncommitted hand edits are set aside and restored. It never creates a merge
    commit.
  - After the pull the in-memory index is rebuilt from the files, so placement, duplicate checks and
    `rev` checks see the other machine's atoms, hunches and sources. Because every run commits and pushes
    immediately, copies rarely diverge, and a batch `add` running on two machines is safe: the
    second run finds the same source hash and writes nothing.
  - Conflicts are rare by design: one claim per file, ids derived from source and locator. When one
    happens anyway (the same file changed on both sides), the rebase is aborted, the vault is left
    exactly as it was, nothing is written, and the run stops with `CONFLICT` (exit 4) and a hint naming
    the files. slowhunch never resolves a conflict itself and never picks a side.
  - An unreachable remote (offline) does not block work: the run continues locally, the report carries
    `PULL_FAILED`, and the next run rebases and pushes. `git.require_pull: true` turns that into a
    stop for owners who prefer never to diverge.
  - `--dry-run` never pulls, so the vault stays byte-identical; it only reports `BEHIND` when the
    remote has commits that are not yet local. Read-only commands (`search`, `get`, `list`, `stats`)
    do not pull unless called with `--sync`.
  - `slowhunch sync` does pull then push on demand, with nothing else, for a quick refresh at the start
    of a session on another machine. The `git` key of every report says what was pulled (commit count)
    and pushed.
  - Config: `git.pull` (default true), `git.require_pull` (default false); `--no-pull` overrides one run.
- **An agent never invents.** No medium, locator, gloss, hunch or tag unsupported by the input or the
  owner's message. Uncertain is a `--dry-run` and a question, not a guess.

## Agent interface

- **CRUD:** `create`, `get`, `update` (field patch or body append), `delete` (soft, to `archive/`,
  `--hard --yes` to remove), `list` with filters.
- **Links and graph:** `link`, `unlink`, `backlinks`, `graph <id> --depth N`.
- **Hunches:** `hunch add`, `hunch evidence`, `hunch state`, `hunch due`.
- **Reading an aesthetic item:** `source facts <id>` returns the stored excerpt with stable line
  numbers, so an agent can re-derive a `locator` without re-reading the original and can re-read the same text
  months later without the page having moved.
- **Search is the point, so it is one call and ranked.** `search` is TF-IDF (stdlib) plus filters, but
  it takes `--medium`, `--stance`, `--tag`, `--hunch` and a `--budget` in tokens, and returns atoms
  ranked by score, recency and hops from the question, each with quote, locator and source. An agent
  answering a question composes one call and reads; it never lists a folder and skims. Two defaults
  matter: a `reads` hit comes with the atom it `rereads`, and a hit whose source `translates` another
  source comes with its counterpart, so a question about a line always shows the disagreement the book
  holds about it.
- **Safety:** every write is atomic, confined to the vault (symlink-resolved path check), checked with
  optimistic concurrency (`rev`; a stale `rev` returns the current content and a conflict error), and
  logged in `audit.log`. Every change is committed and, when a remote exists, pushed (see
  Management); never force-push.
- **Output:** every command has `--json` with a versioned schema, machine-readable errors
  (`code`, `message`, `hint`) and fixed exit codes (0 ok, 1 config/fatal, 2 usage, 3 lint failure,
  4 conflict). The `add` report keeps the order in step 7 as array keys — `contradictions`,
  `hunches_moved`, `atoms`, `corroborations`, `echoes`, `wildcards`, `collisions`, `counts`, `errors`,
  `cost`, `git` — so a client renders it without sorting, and an empty key means "nothing there", never
  "unknown".
- `slowhunch skill` prints a short guide an agent can load: data model, rules, example call sequences,
  and the session shape below. It fits in one screen so it can be injected at every session start.
- `slowhunch verify` (integrity pass, above) and `slowhunch stats --json` (what the book is made of, by
  `medium` and `stance`) are the two commands an agent runs first in an unfamiliar vault. `stats` also
  reports the `asserts`/`reads` ratio, so a book sliding into pure taste collection is visible without
  reading it.

## What a session looks like

The agent is the only interface; there is no UI to maintain. The owner adds things and asks the book
questions. The agent's work sits between those two verbs:

- **Add:** the owner sends a link or pastes text and says why. The agent reads the link itself, hands slowhunch the text, which gates, extracts,
  places, writes thin files and reports. If the owner sends a poem or a picture with no explanation, the
  agent asks one short question about intent (usually "which hunch?") before extracting, because the
  answer is the difference between an atom and a sticker. It never asks more than one question per item.
- **Ask:** the owner asks "what do I have about ruins?" or "connect these". The agent runs one `search`,
  follows `backlinks` where it helps, and answers with `[[id]]`s and exact quotes; nothing it says is
  stored until the owner says so. It answers from the book only, and says so when the book is thin.
- **Steer:** the owner opens a hunch, or passes `--hunch`, and the book bends toward it. An agent that
  sees the same motif three times proposes a hunch rather than waiting to be asked; a proposed hunch is a
  `create` with `state: open` and no other ceremony. This is the step that turns a hoard into a book:
  hunches are the only thing the owner is ever asked to steer, and an agent should be biased toward
  opening one.
- **Nothing is stored as prose.** The answer to a question lives in the chat history; the book keeps
  only atoms, sources, hunches, proposals and topic appends, so a question never creates something the
  owner has to maintain. If the owner says "keep that", the answer is appended to the nearest topic with
  its `[[id]]`s and its date, and does not become a new file.

### A worked exchange

```
owner: add this — the Larkin line about the trees. hunch: why plain diction ages well
agent: add "The trees are coming into leaf / Like something almost being said"
         --as poem --author "Philip Larkin" --hunch why-plain-diction-ages-well
  -> source poem-larkin-trees, inbox copy numbered
  -> crafts atom, locator lines 1-2 (enjambment held flat against the simile)
  -> reads atom,  same locator, no `rereads` on a first reading
  -> placed: extends note:plain-diction-is-not-simplicity -> echo
  -> hunch evidence appended: 2026-11-04, supports
  report: 2 atoms, 0 dropped, 1 echo, 1 hunch moved, 412 tokens
```

Every `[[id]]` in a report exists by the time the report prints. Nothing in it is prose to tidy.

## Security and privacy

- Intake text is **untrusted**: fenced in prompts, delimiter look-alikes neutralized, never followed
  as instructions, never executed, slugs sanitized.
- Outgoing HTTP only to the inference provider hosts listed in config, with timeouts, response size caps
  and backoff on 429/5xx. slowhunch makes no other network requests. `exec` providers run with a
  timeout and a scrubbed environment.
- Secrets only from env or a `0600` env file; never logged or written to the vault, reports or state;
  redacted in errors. A note containing a secret-like pattern is never sent to a provider.
- `send_policy` in config says which vault folders and how many characters may go to a provider; the
  default sends no vault content except the capped neighbour excerpts the place step needs from `notes/`
  and `topics/`. `inbox/` and folders marked private are never sent.
- **Aesthetic text is the same material under copyright as any article**, so the same rules hold: the
  excerpt in `inbox/` may go to a provider, the whole poem is never mirrored to disk, and a work longer
  than the configured cap stays a citation plus its quoted lines. A `send_policy` level `local_only`
  keeps a marked folder on a local provider.
- slowhunch stores text and short quotes, not full copies of pages or media, and never stores or uses
  cookies, sessions or credentials.
- Tests and smoke runs use temporary vaults only; a guard refuses paths listed in
  `SLOWHUNCH_PROTECTED` when `SLOWHUNCH_TEST=1`.

## Tech stack **(default)**

Go 1.27, standard library only (third-party packages only when a requirement cannot be met without one,
each justified), plain files plus git (the `git` binary is needed for auto-commit and push), no database, no web UI. The only stdlib gap is text search
quality, handled by a small optional binary index later, not by adding a dependency now. In-memory index
rebuilt from files, optionally cached in `.slowhunch/cache/` (safe to delete). Optional external
tools, none required for core use: `make`, systemd for scheduling. License MIT.

## Documentation to deliver

A stranger should succeed in ten minutes. `README.md` opens with what slowhunch is, a six-line
quickstart that adds one article and one poem, a diagram of the flow (input, read, gate, extract, place,
write, report), and links to:

- `docs/setup.md`: requirements, install, `slowhunch init`, add a private git remote, configure a provider, first `add` and
  first dry run, scheduling (systemd and cron), uninstall, troubleshooting.
- `docs/inputs.md`: each input type with examples (pasted text, file, batch file, and pasted
  poem, lyric, quotation or art description), how an agent hands over a page it read (`--url` as citation),
  known limits and failure codes, and the text-only, excerpt-first rules for images and songs.
- `docs/concepts.md`: atoms and their three stances, hunches, proposals, the seven patterns, why there
  is no prose layer, and worked examples for a poem, a painting and a paper side by side.
- `docs/vault-format.md`: every type and field with examples, including `medium`, `stance`, `locator`,
  `rereads`, `rendering`, `translates`, `owner_note`, and one complete valid file of every type.
- `docs/providers.md`: the interface, the three built-ins, three ways to add your own (config only for
  OpenAI-compatible, an `exec` script, a Go package), with copy-paste examples.
- `docs/agent-guide.md`: CLI `--json` usage, example agent
  sessions, rules for safe writes, the `verify`/`--dry-run` habit, and the standing rules for aesthetic
  material (quote exactly, emit a locator, keep the original language in `evidence`, gloss in
  `rendering`, never summarize a poem, prefer `reads` over paraphrase).
- `docs/prompts.md`: the three prompt files, how to version and diff them, how to tune per `medium`
  without touching Go, and the expected failure modes of each.
- `docs/security.md`: threat model, what leaves the machine (including what a push to the remote publishes: the vault, `inbox/` excerpts
  included, so use a private remote), copyright handling of
  excerpts vs. full works.
- `docs/management.md`: prompts as versioned data, `verify`, backups, git habits (including working from several machines), what to do when a
  model or a provider changes, and how to read `stats` without fooling yourself.
- `CONTRIBUTING.md`, `LICENSE`.

Every command shown in the docs is exercised by a test so the docs cannot go stale. Example data in
`examples/vault/` is fake.

## Success criteria

- A user can run `slowhunch add "pasted text"`, `add -` (stdin), `add <file>` and
  `search "<query>"` and get atoms with verbatim evidence; an input with no text fails cleanly with a
  code (`BAD_INPUT` or `UNSUPPORTED`) and a hint.
- A pasted poem or song lyric added with `--as poem|lyric` yields at least one `crafts` or `reads`
  atom with an exact excerpt and a valid locator; a second `add` of the same poem yields a linked
  `rereads` atom, not a folded corroboration; a mismatched line is dropped and counted.
- An agent can complete this sequence through the CLI `--json`: create an atom, link
  it, add a hunch, attach evidence, soft-delete a note, run lint, with no conflicting overwrite.
- Swapping the inference provider or adding a new one needs config or a script only, no Go changes.
- Every vault-changing command leaves one commit of exactly the files it wrote, pushed when a remote
  exists; a failed push is reported as `PUSH_FAILED` and retried by the next run; `--dry-run` commits
  nothing.
- With the vault cloned on two machines, a change made and pushed on one is pulled before the next
  write on the other; a same-file conflict stops the run with `CONFLICT` and leaves the vault unchanged.
- A fabricated quote is dropped; a duplicate appends a corroboration without a new file; a
  contradiction creates a flagged atom and hunch evidence; `--dry-run` leaves the vault
  byte-identical.
- After a month of use `slowhunch stats` shows atoms added, hunches opened/moved/dropped, proposals
  promoted, contradictions and echoes found, counters split by `stance` and `medium`, the
  `asserts`/`reads` ratio, and token cost per provider and role.
- `slowhunch verify` on a hand-corrupted vault reports the broken atom by `id` and line, and changes
  nothing. A poem whose stanza was silently truncated by a provider call is reported, not accepted.
- `gofmt`, `go vet`, `go test ./...` and `go build ./...` all pass.

## Out of scope for the first release

Web UI; any fetching of content by slowhunch itself (URLs, web search, platform sites, feeds, scheduled
ingest: the agent reads and passes text in); an MCP server (agents use the CLI; a thin wrapper can come later);
posting, commenting or any write to a platform; crawling or following outlinks; logging in for the user
or storing credentials; downloading or storing media; a vision or audio model (an image is
described in text by the owner, a song enters through its lyric); OCR, scores or chords; PDF OCR and
full-text paper analysis (plain-text extraction only); embeddings or a vector database (search stays
TF-IDF, which is weak on verse and is the first thing to revisit); multi-user sync; a daemon; importing
an existing vault (a migration note in the docs is enough).

## Open assumptions for the owner to confirm

Go and stdlib-only; Markdown plus git storage; CLI only (no MCP server in the first release); MIT license; vault in a separate
repo; built-in providers `openai-compatible`, `anthropic`, `exec`; no fetchers (the agent reads
links and searches and passes the text in); atom cap 5 per `asserts` input and 3 per
aesthetic input; 10% wildcard
share; 25% of every `collide` run reserved for cross-medium pairs; `send_policy` default `excerpt_only`; auto-commit, auto-push and pull-before-write on by default.

Unconfirmed and worth your word: the aesthetic path treats a poem and a paper as the same object with a
different gate, which is the whole bet. If it turns out poems need a *different* store rather than the
same graph, the `reads` and `echoes` machinery is where to split. The other bet is TF-IDF over verse:
it will be visibly bad on short lines within a month, so budget the first search change early.
