package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("list", "list entries with optional filters", cmdList)
}

type listItem struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Path  string `json:"path"`
	Title string `json:"title"`
}

type listResult struct{ Items []listItem }

func (l listResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"items": l.Items, "count": len(l.Items)})
}

func (l listResult) Text() string {
	var b strings.Builder
	for _, it := range l.Items {
		fmt.Fprintf(&b, "%s  %s  %s\n", it.ID, it.Type, it.Title)
	}
	return b.String()
}

func cmdList(env *Env, args []string) (any, error) {
	fs := newFlagSet("list")
	typ := fs.String("type", "", "entry type")
	medium := fs.String("medium", "", "medium")
	stance := fs.String("stance", "", "note stance")
	tag := fs.String("tag", "", "tag")
	hunch := fs.String("hunch", "", "hunch id in bears_on")
	state := fs.String("state", "", "hunch state")
	limit := fs.Int("limit", 0, "max items (0 = no limit)")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "list takes no positional arguments", "usage: slowhunch list [-type T] [-medium M] ...")
	}
	check := func(name, val string, allowed []string) error {
		if val != "" && !slices.Contains(allowed, val) {
			return errs.New(errs.BadInput, fmt.Sprintf("unknown %s %q", name, val), "one of: "+strings.Join(allowed, " "))
		}
		return nil
	}
	for _, c := range []struct {
		n, v string
		a    []string
	}{{"type", *typ, vault.Types}, {"medium", *medium, vault.Mediums}, {"stance", *stance, vault.Stances}, {"state", *state, vault.HunchStates}} {
		if err := check(c.n, c.v, c.a); err != nil {
			return nil, err
		}
	}
	if *limit < 0 {
		return nil, errs.New(errs.BadInput, "limit must not be negative", "use 0 for no limit")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	items := []listItem{}
	for _, e := range ix.All() {
		if *typ != "" && e.Type != *typ {
			continue
		}
		if *medium != "" && ((e.Type != "note" && e.Type != "source") || ix.MediumOf(e.ID) != *medium) {
			continue
		}
		if *stance != "" && (e.Type != "note" || e.Doc.GetString("stance") != *stance) {
			continue
		}
		if *tag != "" && !slices.Contains(e.Doc.GetList("tags"), *tag) {
			continue
		}
		if *hunch != "" && !slices.Contains(e.Doc.GetList("bears_on"), *hunch) {
			continue
		}
		if *state != "" && (e.Type != "hunch" || e.Doc.GetString("state") != *state) {
			continue
		}
		var title string
		switch e.Type {
		case "note":
			title = e.Doc.GetString("claim")
		case "hunch":
			title = e.Doc.GetString("question")
		case "proposal":
			title = e.Doc.GetString("idea")
		default:
			title = e.Doc.GetString("title")
		}
		items = append(items, listItem{e.ID, e.Type, e.Path, title})
		if *limit > 0 && len(items) >= *limit {
			break
		}
	}
	return listResult{items}, nil
}
