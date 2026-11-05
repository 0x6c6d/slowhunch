#!/usr/bin/env bash
# Verification for T025 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "report tests" <<'CMD'
gotest "./internal/report/..." 'TestReport' TestReportKeyOrder TestReportIDs TestReportSaveLoad TestReportAddCost
CMD
expect_ok "probe: the 11 keys appear in spec order" <<'CMD'
goprobe <<'GO'
package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/report"
)

func main() {
	r := report.New("20261105-120000-abcdef")
	r.Atoms = append(r.Atoms, report.AtomRef{ID: report.Link("a-1"), Action: "new"})
	b, _ := json.Marshal(r)
	s := string(b)
	last := -1
	for _, k := range []string{"contradictions", "hunches_moved", "atoms", "corroborations", "echoes", "wildcards", "collisions", "counts", "errors", "cost", "git"} {
		i := strings.Index(s, "\""+k+"\":")
		if i < 0 || i < last {
			os.Exit(1)
		}
		last = i
	}
	if ids := r.IDs(); len(ids) != 1 || ids[0] != "a-1" {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: an empty report has no null values" <<'CMD'
goprobe <<'GO'
package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/report"
)

func main() {
	b, err := json.Marshal(report.New("x"))
	if err != nil {
		os.Exit(125)
	}
	if strings.Contains(string(b), "null") {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
