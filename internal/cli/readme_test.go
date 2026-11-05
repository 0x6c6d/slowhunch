package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestReadmeQuickstart(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	i := strings.Index(readme, "\n## Quick start")
	if i < 0 {
		t.Fatal("no Quick start heading")
	}
	rest := readme[i:]
	const open = "```sh\n"
	s := strings.Index(rest, open)
	if s < 0 {
		t.Fatal("no sh block")
	}
	rest = rest[s+len(open):]
	e := strings.Index(rest, "\n```\n")
	if e < 0 {
		t.Fatal("unterminated sh block")
	}
	script := rest[:e+1]
	m := regexp.MustCompile(`(?m)^slowhunch init (\S+)`).FindStringSubmatch(script)
	if m == nil {
		t.Fatal("no slowhunch init line")
	}
	dir := m[1]

	tmp := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := json.Marshal(map[string]any{"provider": map[string]any{
		"name": "exec", "command": []string{exe, "slowhunch-fake-model"}}})
	cfgDir := filepath.Join(tmp, dir, ".slowhunch")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(tmp, "bin", "slowhunch")
	build := exec.Command("go", "build", "-o", bin, "./cmd/slowhunch")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-e", "-c", script)
	cmd.Dir = tmp
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SLOWHUNCH_VAULT=") || strings.HasPrefix(kv, "PATH=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, "PATH="+filepath.Join(tmp, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("quickstart failed: %v\n%s", err, out)
	}

	var poem, article bool
	filepath.Walk(filepath.Join(tmp, dir, "sources"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		txt := string(b)
		poem = poem || strings.Contains(txt, "medium: poem")
		article = article || strings.Contains(txt, "medium: article")
		return nil
	})
	if !poem || !article {
		t.Fatalf("sources missing: poem=%v article=%v", poem, article)
	}
}
