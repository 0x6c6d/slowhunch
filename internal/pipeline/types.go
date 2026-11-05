// Package pipeline holds the per-item steps between intake and the vault.
package pipeline

import (
	"strings"

	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Item is one intake item moving through the pipeline.
type Item struct {
	SourceID, Medium, Title, Author, URL, Note, Hunch string
	Stored                                            []byte // inbox bytes as stored (vault.FormatInbox output)
	Plain                                             string // vault.InboxPlain(Stored, Medium): ground truth for evidence checks
	Delim                                             string // per-run delimiter from intake.NewDelimiter
	Force                                             bool
}

// Class returns "owner", "aesthetic" or "factual".
func Class(medium string, hasNote, hasHunch bool) string {
	if medium == "own" || medium == "other" || hasNote || hasHunch {
		return "owner"
	}
	if vault.IsAesthetic(medium) {
		return "aesthetic"
	}
	return "factual"
}

// Header returns plain metadata lines that come from the agent, not the intake text.
func Header(it Item) string {
	var lines []string
	add := func(k, v string) {
		if v != "" {
			lines = append(lines, k+": "+v)
		}
	}
	add("medium", it.Medium)
	add("title", it.Title)
	add("author", it.Author)
	add("owner note", it.Note)
	return strings.Join(lines, "\n")
}

// FencedText returns the stored text wrapped in the item's delimiter fence.
func FencedText(it Item) string { return intake.Fence(string(it.Stored), it.Delim) }
