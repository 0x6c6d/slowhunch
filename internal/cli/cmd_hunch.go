package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("hunch add", "add a hunch (an open question)", cmdHunchAdd)
	register("hunch evidence", "attach a note to a hunch as evidence", cmdHunchEvidence)
	register("hunch state", "set the state of a hunch", cmdHunchState)
	register("hunch due", "list hunches due for revisit", cmdHunchDue)
}

func cmdHunchAdd(env *Env, args []string) (any, error) {
	fs := newFlagSet("hunch add")
	next := fs.String("next-test", "", "next test to run")
	revisit := fs.String("revisit", "", "revisit date YYYY-MM-DD")
	tags := fs.String("tags", "", "comma-separated tags")
	author := fs.String("author", "human", "author")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := `usage: slowhunch hunch add "<question>" [-next-test T] [-revisit YYYY-MM-DD] [-tags a,b] [-author A]`
	if len(pos) != 1 || strings.TrimSpace(pos[0]) == "" {
		return nil, errs.New(errs.BadInput, "hunch add needs a non-empty question", usage)
	}
	question := strings.TrimSpace(pos[0])
	v, cfg, err := env.openRun()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	id := vault.UniqueID(vault.KebabID(question), func(c string) bool {
		if _, ok := ix.Get(c); ok {
			return true
		}
		return v.Exists("hunches/" + c + ".md")
	})
	today := env.today()
	d := &frontmatter.Doc{}
	add := func(k string, val any) { d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: val}) }
	add("id", id)
	add("type", "hunch")
	add("created", today)
	add("updated", today)
	add("author", *author)
	add("rev", "000000")
	if *tags != "" {
		var tl []string
		for _, t := range strings.Split(*tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tl = append(tl, t)
			}
		}
		if len(tl) > 0 {
			add("tags", tl)
		}
	}
	add("question", question)
	add("state", "open")
	if *next != "" {
		add("next_test", *next)
	}
	if *revisit != "" {
		add("revisit", *revisit)
	}
	add("evidence", []map[string]string{})
	if err := vault.Validate(d, id); err != nil {
		return nil, err
	}
	txn, err := env.begin(v, cfg, "hunch add")
	if err != nil {
		return nil, err
	}
	rel := "hunches/" + id + ".md"
	rev, err := txn.PutDoc(rel, d, "")
	if err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{id}, "hunches: 1")
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "path": rel, "rev": rev, "git": map[string]any{"commit": hash}}, nil
}

func cmdHunchEvidence(env *Env, args []string) (any, error) {
	fs := newFlagSet("hunch evidence")
	noteID := fs.String("note", "", "note id")
	relation := fs.String("relation", "", "supports or weakens")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch hunch evidence <id> -note <note-id> -relation supports|weakens"
	if len(pos) != 1 || *noteID == "" {
		return nil, errs.New(errs.BadInput, "hunch evidence needs <id> and -note", usage)
	}
	if *relation != "supports" && *relation != "weakens" {
		return nil, errs.New(errs.BadInput, fmt.Sprintf("bad relation %q", *relation), "one of: supports weakens")
	}
	v, cfg, err := env.openRun()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	he, err := getHunch(ix, pos[0])
	if err != nil {
		return nil, err
	}
	ne, ok := ix.Get(*noteID)
	if !ok || ne.Type != "note" {
		return nil, errs.New(errs.NotFound, "no note with id "+*noteID, "try slowhunch search")
	}
	rev := frontmatter.Rev(he.Doc)
	d := he.Doc.Clone()
	ev := append([]map[string]string{}, he.Doc.GetMaps("evidence")...)
	ev = append(ev, map[string]string{"date": env.today(), "note": *noteID, "relation": *relation})
	d.Set("evidence", ev)
	d.Set("updated", env.today())
	state := d.GetString("state")
	if state == "open" {
		state = "warming"
		d.Set("state", state)
	}
	if err := vault.Validate(d, he.ID); err != nil {
		return nil, err
	}
	txn, err := env.begin(v, cfg, "hunch evidence")
	if err != nil {
		return nil, err
	}
	if _, err := txn.PutDoc(he.Path, d, rev); err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{he.ID}, "evidence: 1")
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": he.ID, "evidence_count": len(ev), "state": state, "git": map[string]any{"commit": hash}}, nil
}

func getHunch(ix *vault.Index, id string) (*vault.Entry, error) {
	e, ok := ix.Get(id)
	if !ok {
		return nil, errs.New(errs.NotFound, "no hunch with id "+id, "try slowhunch list -type hunch")
	}
	if e.Type != "hunch" {
		return nil, badInput(id + " is not a hunch")
	}
	return e, nil
}

func cmdHunchState(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("hunch state"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 2 {
		return nil, errs.New(errs.BadInput, "hunch state needs <id> and <state>", "usage: slowhunch hunch state <id> <state>")
	}
	if !slices.Contains(vault.HunchStates, pos[1]) {
		return nil, errs.New(errs.BadInput, fmt.Sprintf("unknown state %q", pos[1]), "one of: "+strings.Join(vault.HunchStates, " "))
	}
	v, cfg, err := env.openRun()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	he, err := getHunch(ix, pos[0])
	if err != nil {
		return nil, err
	}
	rev := frontmatter.Rev(he.Doc)
	d := he.Doc.Clone()
	d.Set("state", pos[1])
	d.Set("updated", env.today())
	if err := vault.Validate(d, he.ID); err != nil {
		return nil, err
	}
	txn, err := env.begin(v, cfg, "hunch state")
	if err != nil {
		return nil, err
	}
	if _, err := txn.PutDoc(he.Path, d, rev); err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{he.ID}, "state: "+pos[1])
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": he.ID, "state": pos[1], "git": map[string]any{"commit": hash}}, nil
}

type dueItem struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Revisit  string `json:"revisit"`
	State    string `json:"state"`
}

type dueResult struct {
	Due   []dueItem
	Today string
}

func (r dueResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"due": r.Due, "today": r.Today})
}

func (r dueResult) Text() string {
	var b strings.Builder
	for _, it := range r.Due {
		fmt.Fprintf(&b, "%s  %s  %s  %s\n", it.Revisit, it.ID, it.State, it.Question)
	}
	return b.String()
}

func cmdHunchDue(env *Env, args []string) (any, error) {
	pos, err := parseFlags(newFlagSet("hunch due"), args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "hunch due takes no arguments", "usage: slowhunch hunch due")
	}
	v, _, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	today := env.today()
	items := []dueItem{}
	for _, e := range ix.OfType("hunch") {
		rv := e.Doc.GetString("revisit")
		st := e.Doc.GetString("state")
		if rv == "" || rv > today || st == "dropped" {
			continue
		}
		items = append(items, dueItem{e.ID, e.Doc.GetString("question"), rv, st})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Revisit != items[j].Revisit {
			return items[i].Revisit < items[j].Revisit
		}
		return items[i].ID < items[j].ID
	})
	return dueResult{items, today}, nil
}
