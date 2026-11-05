package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("source facts", "print the stored text of a source, numbered by line", cmdSourceFacts)
}

type sourceLine struct {
	N    int    `json:"n"`
	Text string `json:"text"`
}

type sourceFactsResult struct {
	ID, Medium string
	Lines      []sourceLine
}

func (s sourceFactsResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"id": s.ID, "medium": s.Medium, "lines": s.Lines, "count": len(s.Lines)})
}

func (s sourceFactsResult) Text() string {
	var b strings.Builder
	for _, l := range s.Lines {
		fmt.Fprintf(&b, "%4d  %s\n", l.N, l.Text)
	}
	return b.String()
}

func cmdSourceFacts(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("source facts"), args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch source facts <id>"
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "source facts needs exactly one id", usage)
	}
	id := pos[0]
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	e, ok := ix.Get(id)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+id, "try slowhunch list -type source")
	}
	if e.Type != "source" {
		return nil, badInput(id + " is not a source")
	}
	medium := e.Doc.GetString("medium")
	stored, err := v.ReadFile("inbox/" + id + ".txt")
	if err != nil {
		if errs.CodeOf(err) == errs.NotFound {
			return nil, errs.New(errs.NotFound, "no stored text for "+id, "this source has no stored text")
		}
		return nil, err
	}
	lines := []sourceLine{}
	for i, t := range vault.InboxLines(stored, medium) {
		lines = append(lines, sourceLine{N: i + 1, Text: t})
	}
	return sourceFactsResult{ID: id, Medium: medium, Lines: lines}, nil
}
