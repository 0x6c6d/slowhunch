package cli

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func lintTreeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		h.Write([]byte(rel + "\x00"))
		if !d.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(h.Sum(nil))
}

func lintNoteFile(id, created, extra string) string {
	return "---\nid: " + id + "\ntype: note\ncreated: " + created + "\nclaim: c\n" + extra + "---\nbody\n"
}

func TestLintCommand(t *testing.T) {
	d := newVault(t)
	rc, m, s := runCmd(d, "lint")
	if rc != 0 || m["ok"] != true || len(m["failures"].([]any)) != 0 {
		t.Fatalf("clean: rc=%d %s", rc, s)
	}
	cases := []struct{ id, code, body string }{
		{"v-loc", "MISSING_LOCATOR", "stance: crafts\nsource: poem\nevidence: e\nlocator: \"\"\n"},
		{"v-conf", "CONFIDENCE_NOT_ASSERTS", "stance: crafts\nsource: paper\nevidence: e\nlocator: l\nconfidence: \"0.5\"\n"},
		{"v-rend", "RENDERING_AS_EVIDENCE", "stance: asserts\nsource: paper\nevidence: e\nlocator: l\nrendering: e\n"},
		{"v-reads", "READS_WITHOUT_REREADS", ""},
	}
	for _, c := range cases {
		d := newVault(t)
		mustCreate(t, d, "source", `{"medium":"poem","title":"P","author":"A"}`)
		write := func(id, content string) {
			if err := os.WriteFile(filepath.Join(d, "notes", id+".md"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		// sources must exist for the note's source field
		for _, sd := range []struct{ id, medium string }{{"poem", "poem"}, {"paper", "paper"}} {
			if err := os.WriteFile(filepath.Join(d, "sources", sd.id+".md"), []byte("---\nid: "+sd.id+"\ntype: source\nmedium: "+sd.medium+"\ntitle: t\n---\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if c.code == "READS_WITHOUT_REREADS" {
			write("v-a", lintNoteFile("v-a", "2026-01-01", "stance: asserts\nsource: paper\nevidence: same\nlocator: l\n"))
			write("v-reads", lintNoteFile("v-reads", "2026-01-02", "stance: reads\nsource: paper\nevidence: same\nlocator: l\n"))
		} else {
			write(c.id, lintNoteFile(c.id, "2026-01-01", c.body))
		}
		before := lintTreeHash(t, d)
		rc, m, s := runCmd(d, "lint")
		if rc != 3 {
			t.Fatalf("%s: rc=%d %s", c.code, rc, s)
		}
		found := false
		for _, f := range m["failures"].([]any) {
			fm := f.(map[string]any)
			if fm["id"] == c.id && fm["code"] == c.code {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: failure missing: %s", c.code, s)
		}
		if lintTreeHash(t, d) != before {
			t.Errorf("%s: vault changed", c.code)
		}
	}
}
