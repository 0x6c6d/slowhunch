#!/usr/bin/env bash
# Verification for T030 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "intake read tests" <<'CMD'
gotest "./internal/intake/..." 'TestIntakeRead|TestIntakeCheck|TestInferMedium' TestIntakeRead TestIntakeCheck TestInferMedium
CMD
expect_fail "negative: pdf, too long, empty, art without text are refused with the right codes" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/intake"
)

func main() {
	dir, _ := os.MkdirTemp("", "inprobe")
	defer os.RemoveAll(dir)
	pdf := filepath.Join(dir, "paper.pdf")
	os.WriteFile(pdf, []byte("%PDF-1.4"), 0o644)
	if _, err := intake.Read(pdf, strings.NewReader("")); errs.CodeOf(err) != errs.Unsupported {
		os.Exit(0)
	}
	ok := intake.Input{Text: strings.Repeat("é", 12000)}
	if intake.Check(ok, "article", "", "") != nil {
		os.Exit(125)
	}
	cases := []struct {
		in     intake.Input
		medium string
		title  string
		code   string
	}{
		{intake.Input{Text: strings.Repeat("a", 12001)}, "article", "", errs.TooLong},
		{intake.Input{Text: "  \n "}, "article", "", errs.BadInput},
		{intake.Input{Text: ""}, "art", "T", errs.Unsupported},
		{intake.Input{Text: "a scene"}, "film", "", errs.BadInput},
	}
	for _, c := range cases {
		if errs.CodeOf(intake.Check(c.in, c.medium, c.title, "")) != c.code {
			os.Exit(0)
		}
	}
	os.Exit(1)
}
GO
CMD
finish
