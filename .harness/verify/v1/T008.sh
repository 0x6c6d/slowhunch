#!/usr/bin/env bash
# Verification for T008 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "write path tests" <<'CMD'
gotest "./internal/vault/..." 'TestWriteConfinement|TestAtomicWrite|TestConflictRev|TestEvidenceImmutable|TestInboxImmutable|TestTestGuard|TestAuditAndState' TestWriteConfinement TestAtomicWrite TestConflictRev TestEvidenceImmutable TestInboxImmutable TestTestGuard TestAuditAndState
CMD
expect_ok "probe: confined write, PutDoc rev round trip" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "vprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	v, err := vault.Open(dir)
	if err != nil || v.WriteFile("notes/a.txt", []byte("x")) != nil {
		os.Exit(1)
	}
	d := &frontmatter.Doc{Body: "b\n"}
	d.Set("id", "t-1")
	r1, err := v.PutDoc("topics/t-1.md", d, "")
	if err != nil || len(r1) != 6 {
		os.Exit(1)
	}
	d.Set("id", "t-1")
	d.Body = "c\n"
	if _, err := v.PutDoc("topics/t-1.md", d, r1); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: dot-dot and symlink escapes are refused and nothing lands outside" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	base, _ := os.MkdirTemp("", "vprobe")
	defer os.RemoveAll(base)
	root := filepath.Join(base, "v")
	out := filepath.Join(base, "outside")
	os.MkdirAll(filepath.Join(root, ".slowhunch"), 0o755)
	os.MkdirAll(out, 0o755)
	os.Symlink(out, filepath.Join(root, "notes"))
	v, err := vault.Open(root)
	if err != nil {
		os.Exit(125)
	}
	e1 := v.WriteFile("../outside/a.txt", []byte("x"))
	e2 := v.WriteFile("notes/b.txt", []byte("x"))
	e3 := v.WriteFile("/tmp/abs.txt", []byte("x"))
	ents, _ := os.ReadDir(out)
	if e1 == nil || e2 == nil || e3 == nil || len(ents) != 0 {
		os.Exit(0) // an escape succeeded
	}
	os.Exit(1)
}
GO
CMD
expect_fail "negative: stale rev is refused with CONFLICT_REV" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "vprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, ".slowhunch"), 0o755)
	v, _ := vault.Open(dir)
	d := &frontmatter.Doc{}
	d.Set("id", "t-1")
	if _, err := v.PutDoc("topics/t-1.md", d, ""); err != nil {
		os.Exit(125)
	}
	d.Set("title", "x")
	_, err := v.PutDoc("topics/t-1.md", d, "000000")
	if err == nil {
		os.Exit(0)
	}
	if errs.CodeOf(err) != errs.ConflictRev {
		os.Exit(125)
	}
	os.Exit(1)
}
GO
CMD
expect_fail "negative: test guard refuses a protected vault path" <<'CMD'
goprobe <<'GO'
package main

import (
	"os"
	"path/filepath"

	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	dir, _ := os.MkdirTemp("", "vprobe")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, "v", ".slowhunch"), 0o755)
	os.Setenv("SLOWHUNCH_TEST", "1")
	os.Setenv("SLOWHUNCH_PROTECTED", dir)
	if _, err := vault.Open(filepath.Join(dir, "v")); err == nil {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
