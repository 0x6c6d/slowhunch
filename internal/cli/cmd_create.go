package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("create", "create a note, source, hunch, proposal or topic from JSON", cmdCreate)
}

func cmdCreate(env *Env, args []string) (any, error) {
	fs := newFlagSet("create")
	data := fs.String("data", "", "JSON object, or - for stdin")
	idFlag := fs.String("id", "", "explicit id")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch create <type> --data <json|-> [--id ID]"
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "create needs exactly one type", usage)
	}
	typ := pos[0]
	if vault.Folder(typ) == "" {
		return nil, errs.New(errs.BadInput, "unknown type: "+typ, "one of note, source, hunch, proposal, topic")
	}
	if *data == "" {
		return nil, errs.New(errs.BadInput, "--data is required", usage)
	}
	raw := []byte(*data)
	if *data == "-" {
		raw, err = io.ReadAll(env.Stdin)
		if err != nil {
			return nil, badInput(err.Error())
		}
	}
	var fields map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&fields); err != nil || fields == nil {
		return nil, errs.New(errs.BadInput, "data must be a JSON object", usage)
	}
	if dec.More() {
		return nil, badInput("data has trailing content")
	}
	for _, k := range []string{"id", "type", "created", "updated", "rev"} {
		if _, ok := fields[k]; ok {
			return nil, badInput("field " + k + " is set by slowhunch and cannot be given")
		}
	}
	body := ""
	if b, ok := fields["body"]; ok {
		s, ok := b.(string)
		if !ok {
			return nil, badInput("body must be a string")
		}
		body = s
		delete(fields, "body")
	}
	in, err := docFromJSON(fields)
	if err != nil {
		return nil, err
	}

	v, cfg, err := env.openRun()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	str := in.GetString
	id := *idFlag
	if id != "" {
		if id != vault.Slug(id, 120) {
			return nil, badInput("--id must be lowercase kebab-case: " + vault.Slug(id, 120))
		}
	} else {
		switch typ {
		case "hunch":
			id = vault.KebabID(str("question"))
		case "proposal":
			id = vault.KebabID(str("idea"))
		case "topic":
			id = vault.Slug(str("title"), 60)
		case "source":
			id = vault.SourceID(str("medium"), str("author"), str("title"), str("title")+str("url"))
		case "note":
			id = vault.AtomID(str("stance"), str("source"), str("locator"), str("claim"))
		}
		if id == "" {
			id = "untitled"
		}
	}
	folder := vault.Folder(typ)
	id = vault.UniqueID(id, func(c string) bool {
		if _, ok := ix.Get(c); ok {
			return true
		}
		return v.Exists(folder + "/" + c + ".md")
	})

	today := env.today()
	writer := "author"
	if typ == "source" {
		writer = "added_by"
	}
	d := &frontmatter.Doc{}
	d.Fields = []frontmatter.Field{{Key: "id", Value: id}, {Key: "type", Value: typ},
		{Key: "created", Value: today}, {Key: "updated", Value: today}}
	if w, ok := in.Get(writer); ok {
		d.Fields = append(d.Fields, frontmatter.Field{Key: writer, Value: w})
	} else {
		d.Fields = append(d.Fields, frontmatter.Field{Key: writer, Value: "human"})
	}
	d.Fields = append(d.Fields, frontmatter.Field{Key: "rev", Value: "000000"})
	defaults := map[string]string{}
	switch typ {
	case "note":
		defaults["status"] = "active"
	case "hunch":
		defaults["state"] = "open"
	case "proposal":
		defaults["status"] = "open"
	}
	for k, val := range defaults {
		if _, ok := in.Get(k); !ok {
			in.Fields = append(in.Fields, frontmatter.Field{Key: k, Value: val})
		}
	}
	sort.Slice(in.Fields, func(i, j int) bool { return in.Fields[i].Key < in.Fields[j].Key })
	for _, f := range in.Fields {
		if f.Key != writer {
			d.Fields = append(d.Fields, f)
		}
	}
	if typ == "note" {
		d.Body = vault.NoteBody(d.GetString("claim"), d.GetString("evidence"))
	} else {
		d.Body = body
	}
	if err := vault.Validate(d, id); err != nil {
		return nil, err
	}

	txn, err := env.begin(v, cfg, "create")
	if err != nil {
		return nil, err
	}
	rel := folder + "/" + id + ".md"
	rev, err := txn.PutDoc(rel, d, "")
	if err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{id}, "created: 1")
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "type": typ, "path": rel, "rev": rev, "git": map[string]any{"commit": hash}}, nil
}
