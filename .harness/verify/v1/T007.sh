#!/usr/bin/env bash
# Verification for T007 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "schema tests" <<'CMD'
gotest "./internal/vault/..." 'TestValidateTypes|TestLocatorGrammar|TestLinks|TestNoteBody|TestInboxFormat' TestValidateTypes TestLocatorGrammar TestLinks TestNoteBody TestInboxFormat
CMD
expect_ok "probe: locators, links, inbox format" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	for _, s := range []string{"line 3", "lines 1-2", "stanza 2", "verse 1", "comment 4", "section Intro", "timecode 1:23-1:31", "region 1,2,30,40"} {
		if _, err := vault.ParseLocator(s); err != nil {
			os.Exit(1)
		}
	}
	l, _ := vault.ParseLocator("lines 1-2")
	if f, t, ok := l.LineRange(); !ok || f != 1 || t != 2 {
		os.Exit(1)
	}
	st := vault.FormatInbox("a\nb", "poem")
	if string(st) != "1\ta\n2\tb\n" || vault.InboxPlain(st, "poem") != "a\nb" || vault.Folder("hunch") != "hunches" {
		os.Exit(1)
	}
	d := &frontmatter.Doc{Body: "see [[x-1]]\n"}
	d.Set("bears_on", []string{"h1"})
	if ls := vault.Links(d); len(ls) != 2 {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: bad locators, unknown field, confidence on crafts are rejected" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func note() *frontmatter.Doc {
	d := &frontmatter.Doc{}
	for _, kv := range [][2]string{{"id", "n-1"}, {"type", "note"}, {"created", "2026-11-05"}, {"updated", "2026-11-05"}, {"author", "human"}, {"rev", "abcdef"}, {"claim", "c"}, {"evidence", "e"}, {"stance", "crafts"}, {"locator", "line 1"}} {
		d.Set(kv[0], kv[1])
	}
	return d
}

func main() {
	if err := vault.Validate(note(), "n-1"); err != nil {
		os.Exit(125) // the valid baseline must pass
	}
	for _, s := range []string{"lines 3-1", "line 0", "page 4", "timecode 1:61-2:00", "region 5,5,1,1", "lines 1-2; rm"} {
		if _, err := vault.ParseLocator(s); err == nil {
			os.Exit(0)
		}
	}
	a := note()
	a.Set("bogus", "x")
	b := note()
	b.Set("confidence", "0.5")
	if vault.Validate(a, "n-1") == nil || vault.Validate(b, "n-1") == nil || vault.Validate(note(), "other-id") == nil {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
