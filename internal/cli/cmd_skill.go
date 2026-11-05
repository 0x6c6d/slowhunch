package cli

import "encoding/json"

func init() {
	register("skill", "print the guide for AI agents", cmdSkill)
}

// skillResult renders the guide as plain text, or as {"text": ...} in JSON mode.
type skillResult struct{}

func (skillResult) Text() string { return skillText }

func (skillResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"text": skillText})
}

func cmdSkill(env *Env, args []string) (any, error) {
	return skillResult{}, nil
}

const skillText = `slowhunch: a commonplace book for agents.

What it is
  A plain Markdown vault kept in git. Every command accepts --json.
  slowhunch never fetches from the network: you read links yourself
  and pass the text in.

Data model
  notes/      atoms, type "note": a one-line claim, a verbatim evidence
              excerpt, stance (asserts|crafts|reads), locator, source.
  sources/    where a work came from. Its text is kept in inbox/.
  hunches/    open questions with dated evidence.
              state: open | warming | tested | dropped.
  proposals/  suggested changes waiting for the owner.
  topics/     groupings of notes and hunches.
  Links are [[id]], one direction. Backlinks are computed.
  Every file has a rev. Updates must pass --rev.

Rules
  - Run slowhunch verify before and after any batch of writes.
  - Write only through the CLI, never by editing vault files.
  - Quote evidence exactly. Never paraphrase a quote.
  - One work or one passage per add.
  - If there is neither a note nor a hunch, ask the owner one short
    question about intent.

Example calls (one command per line)
  slowhunch verify
  slowhunch add - --as poem --title "Trees" --note "why it matters" --hunch <id>
  slowhunch search "query" --limit 5 --json
  slowhunch backlinks <id> --json
  slowhunch get <id> --json
  slowhunch update <id> --rev <rev> --data '{"tags":["x"]}'
  slowhunch hunch add "question"
  slowhunch hunch due

Session shape
  1. slowhunch skill (once)
  2. slowhunch verify
  3. work: add, search, hunch
  4. slowhunch verify
  5. answer the owner with [[id]] links and exact quotes from the book only.
`
