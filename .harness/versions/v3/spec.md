# slowhunch (v3 spec, delta on `.harness/harness.md`)

## 1. Goal
Source text that is itself wiki-style Markdown (e.g. an Obsidian vault) contains `[[...]]` wikilinks. Copied verbatim into `evidence`, they are read by `verify` as slowhunch links and reported as `DANGLING_LINK`, and evidence can never be repaired. v3 removes the brackets at intake, so they never reach `inbox/`, evidence or note bodies.

## 2. New requirements
- **R41.** Wikilink stripping: during intake normalization (R8), only the brackets are removed: every `[[text]]` becomes `text` (so `[[a|b]]` becomes `a|b`) and `![[text]]` becomes `text`. A match is `[[` + 1 or more characters that are not `[`, `]` or a line break + `]]`; the inner text is kept unchanged. Nothing else is changed: single brackets `[x]`, Markdown links `[text](url)`, unbalanced `[[`, and empty `[[]]` stay as they are. The stripping runs for every medium except `code` (assumption: brackets in code are syntax, not wikilinks).
- **R42.** The stripped text is the stored text: `inbox/<source-id>.txt`, `text_hash`, the `<short>` of the source id (R4), the gate/extract prompts and the evidence check (R10) all use it. An `add` of wiki text therefore yields atoms whose evidence and body contain no `[[`, and `verify` reports no `DANGLING_LINK` caused by the source text.

## 3. Changed requirements
R8: was "whitespace normalized, line breaks kept, curly quotes, em dashes and non-breaking spaces normalized once" → now the same plus the wikilink stripping of R41 (not for `code`).

## 4. Removed requirements
none

## 5. Constraints & environment changes
none. Go standard library only, no new packages, tools, secrets or live systems.
Note (not a requirement): files ingested before v3 are not touched. Their evidence still contains the brackets and keeps its `DANGLING_LINK`s. Re-adding such a file creates a new source because its `text_hash` changes (R13 idempotence holds from v3 on).

## 6. Acceptance criteria
- [ ] `go test ./internal/intake/ -run Wikilink` passes with cases: `[[a]]`→`a`, `[[a|b]]`→`a|b`, `![[a]]`→`a`, two links on one line, link with spaces `[[Blood Hound]]`→`Blood Hound`, and unchanged: `[x]`, `[t](u)`, `[[`, `[[]]`, `[[a`+newline+`b]]`. [R41]
- [ ] A test that `Normalize(text, "code")` leaves `m[[i]]` unchanged while the same text with `--as prose` is stripped. [R41]
- [ ] A test that `TextHash` of stripped text differs from the hash of the raw text, and that the stored `inbox/` file has no `[[`. [R42]
- [ ] End-to-end test with the fake provider: `add` of a file containing `... overlap with [[bloodhound]] for AD enumeration ...`, extract returns evidence from the stripped text; the note's `evidence` and body contain `bloodhound` and no `[[`; `slowhunch verify --json` exits 0 with no `DANGLING_LINK`. [R41][R42]
- [ ] End-to-end test: extract returns evidence that still contains `[[bloodhound]]` (raw form): the atom is rejected by the evidence-substring check (R10) and nothing is written. [R42]
- [ ] A vault written before the change (fixture with a note whose evidence contains `[[bloodhound]]`) is not modified by `verify` and still reports `DANGLING_LINK` for it. [R42]
- [ ] A note whose evidence contains `a|b` is written, read back unchanged and passes `verify` (frontmatter round trip). [R41]
- [ ] `README.md` has a sentence on wikilink stripping at intake (`grep -n "wikilink" README.md` prints a line) and says older notes are not rewritten. [R41]
- [ ] `make check` exits 0. [earlier versions keep working]

## 7. Out of scope
Any change to `verify`, link parsing, `update`, `unlink` or the immutability of evidence; repairing or migrating existing notes; stripping inside fenced code blocks of Markdown; other wiki syntax (`#tags`, `[[a#heading]]` anchors stay as the literal text `a#heading`, callouts, frontmatter of the source); stripping in `create`/`keep` text.
