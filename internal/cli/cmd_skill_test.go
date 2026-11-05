package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSkill(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"skill"}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	text := out.String()
	if n := strings.Count(text, "\n"); n > 60 || n < 20 {
		t.Fatalf("guide has %d lines", n)
	}
	for _, line := range strings.Split(text, "\n") {
		if len(line) > 100 {
			t.Fatalf("line too long: %q", line)
		}
	}
	for _, k := range []string{"slowhunch verify", "slowhunch add", "slowhunch search", "--json", "[[id]]"} {
		if !strings.Contains(text, k) {
			t.Fatalf("guide lacks %q", k)
		}
	}

	out.Reset()
	if rc := Run([]string{"skill", "--json"}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["schema"] != float64(1) {
		t.Fatalf("schema: %v", env["schema"])
	}
	if _, ok := env["text"]; !ok {
		t.Fatal("no text key")
	}
}
