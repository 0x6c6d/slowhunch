package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	for _, a := range os.Args {
		if a == "slowhunch-fake-model" {
			fakeModel()
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

var numPrefix = regexp.MustCompile(`^(\d+)\t`)

func fakeModel() {
	var req struct {
		Role, Medium, System, User string
	}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var text string
	switch req.Role {
	case "gate":
		text = `{"keep": true, "reason": "fake"}`
	case "collide":
		text = `{"idea": "fake idea", "cheapest_test": "fake test", "keep": true}`
		// collide-replies=<file>: a JSON map "<id> <id>" -> reply text, used
		// when both ids appear in the request.
		for _, a := range os.Args {
			path, ok := strings.CutPrefix(a, "collide-replies=")
			if !ok {
				continue
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			var replies map[string]string
			if err := json.Unmarshal(raw, &replies); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			for k, r := range replies {
				ids := strings.Fields(k)
				if len(ids) == 2 && strings.Contains(req.User, "id: "+ids[0]+"\n") && strings.Contains(req.User, "id: "+ids[1]+"\n") {
					text = r
				}
			}
		}
	case "place":
		text = `{"verdict": "new", "target": "", "reason": "fake"}`
	default:
		lines := strings.Split(req.User, "\n")
		in := false
		ev, num := "", 0
		for _, l := range lines {
			if strings.HasPrefix(l, "<<<END ") {
				break
			}
			if !in {
				if strings.HasPrefix(l, "<<<SLOWHUNCH-INTAKE-") {
					in = true
				}
				continue
			}
			n := 0
			if mm := numPrefix.FindStringSubmatch(l); mm != nil {
				fmt.Sscan(mm[1], &n)
				l = l[len(mm[0]):]
			}
			if strings.TrimSpace(l) != "" {
				ev, num = l, n
				break
			}
		}
		if num == 0 {
			num = 1
		}
		words := strings.Fields(ev)
		if len(words) > 6 {
			words = words[:6]
		}
		atom := map[string]any{"claim": "Fake claim: " + strings.Join(words, " "), "evidence": ev, "tags": []string{}}
		switch req.Medium {
		case "poem", "lyric", "prose", "quote", "film", "art":
			atom["stance"], atom["locator"] = "crafts", fmt.Sprintf("line %d", num)
		case "code":
			atom["stance"], atom["locator"] = "asserts", fmt.Sprintf("lines %d-%d", num, num)
		case "thread":
			atom["stance"], atom["locator"] = "asserts", "comment 1"
		default:
			atom["stance"], atom["locator"], atom["confidence"] = "asserts", "", 0.5
		}
		b, _ := json.Marshal(map[string]any{"atoms": []any{atom}})
		text = string(b)
	}
	out, _ := json.Marshal(map[string]any{"text": text, "tokens_in": 10, "tokens_out": 5})
	fmt.Println(string(out))
}

func newFakeVault(t *testing.T) string {
	t.Helper()
	d := newVault(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, ".slowhunch", "config.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg["provider"] = map[string]any{"name": "exec", "command": []string{exe, "slowhunch-fake-model"}}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, d, "add", "-A")
	gitOut(t, d, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "fake provider")
	return d
}
