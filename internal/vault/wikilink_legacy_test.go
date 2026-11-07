package vault

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func snapshotVault(t *testing.T, root string) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m[p] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestVerifyLegacyWikilinkUnchanged(t *testing.T) {
	v := idxVault(t, nil)
	putText(t, v, "sources/poem.md", "---\nid: poem\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: P\ntext_hash: abc\n---\n")
	if err := v.WriteFile("inbox/poem.txt", FormatInbox("see [[bloodhound]] here", "poem")); err != nil {
		t.Fatal(err)
	}
	putText(t, v, "notes/legacy.md", verifyNote("legacy", "evidence: \"[[bloodhound]]\"\nlocator: line 1\n")[:len(verifyNote("legacy", "evidence: \"[[bloodhound]]\"\nlocator: line 1\n"))-5]+"> [[bloodhound]]\n")

	before := snapshotVault(t, v.Root)
	ps, _, err := Verify(v)
	if err != nil {
		t.Fatal(err)
	}
	if !codesOf(ps)["legacy"]["DANGLING_LINK"] {
		t.Fatalf("want DANGLING_LINK for legacy, got %v", ps)
	}
	after := snapshotVault(t, v.Root)
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	}
	for p, b := range before {
		if !bytes.Equal(b, after[p]) {
			t.Errorf("%s changed", p)
		}
	}
}

func TestEvidencePipeRoundTrip(t *testing.T) {
	const ev = "a|b and c|d"
	v := idxVault(t, nil)
	putText(t, v, "sources/poem.md", "---\nid: poem\ntype: source\nadded_by: human\ncreated: 2026-01-01\nupdated: 2026-01-01\nauthor: human\nmedium: poem\ntitle: P\ntext_hash: abc\n---\n")
	if err := v.WriteFile("inbox/poem.txt", FormatInbox(ev, "poem")); err != nil {
		t.Fatal(err)
	}
	putText(t, v, "notes/pipe.md", verifyNote("pipe", "evidence: \""+ev+"\"\nlocator: line 1\n"))
	b, err := v.ReadFile("notes/pipe.md")
	if err != nil {
		t.Fatal(err)
	}
	d, err := frontmatter.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.GetString("evidence"); got != ev {
		t.Fatalf("evidence %q != %q", got, ev)
	}
	ps, _, err := Verify(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(codesOf(ps)["pipe"]) != 0 {
		t.Fatalf("pipe problems: %v", ps)
	}
}
