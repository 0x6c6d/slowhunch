package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitMinimal(t *testing.T) {
	d := filepath.Join(t.TempDir(), "v")
	var out, errb bytes.Buffer
	if rc := Run([]string{"init", d, "--json"}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	if !strings.Contains(out.String(), `"schema": 1`) {
		t.Fatal(out.String())
	}
	for _, f := range []string{"notes", "hunches", "sources", "topics", "proposals", "inbox", "archive", ".slowhunch"} {
		if st, err := os.Stat(filepath.Join(d, f)); err != nil || !st.IsDir() {
			t.Fatalf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(d, ".slowhunch", "config.json")); err != nil {
		t.Fatal(err)
	}
	if rc := Run([]string{"init", "--json"}, strings.NewReader(""), &out, &errb); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
}
