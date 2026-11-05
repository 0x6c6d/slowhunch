package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func init() {
	register("update", "patch fields of an entry (with --rev), or append to a topic", cmdUpdate)
}

func cmdUpdate(env *Env, args []string) (any, error) {
	fs := newFlagSet("update")
	revFlag := fs.String("rev", "", "current rev of the entry (from slowhunch get)")
	data := fs.String("data", "", "JSON object patch, or - for stdin")
	appendFlag := fs.String("append", "", "text to append to a topic body, or - for stdin")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := "usage: slowhunch update <id> --rev <rev> (--data <json|-> | --append <text|->)"
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "update needs exactly one id", usage)
	}
	id := pos[0]
	if *revFlag == "" {
		return nil, errs.New(errs.BadInput, "update needs --rev; get the current rev with slowhunch get", usage)
	}
	if (*data == "") == (*appendFlag == "") {
		return nil, errs.New(errs.BadInput, "update needs exactly one of --data or --append", usage)
	}
	readArg := func(s string) (string, error) {
		if s != "-" {
			return s, nil
		}
		b, err := io.ReadAll(env.Stdin)
		if err != nil {
			return "", badInput(err.Error())
		}
		return string(b), nil
	}

	var patch map[string]any
	var appendText string
	if *data != "" {
		raw, err := readArg(*data)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(strings.NewReader(raw))
		if err := dec.Decode(&patch); err != nil || patch == nil {
			return nil, errs.New(errs.BadInput, "data must be a JSON object", usage)
		}
		if dec.More() {
			return nil, badInput("data has trailing content")
		}
		for _, k := range []string{"id", "type", "created", "rev"} {
			if _, ok := patch[k]; ok {
				return nil, badInput("field " + k + " cannot be changed")
			}
		}
	} else {
		raw, err := readArg(*appendFlag)
		if err != nil {
			return nil, err
		}
		appendText = strings.TrimSpace(raw)
		if appendText == "" {
			return nil, badInput("append text must not be empty")
		}
	}

	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	e, ok := ix.Get(id)
	if !ok {
		return nil, errs.New(errs.NotFound, "no entry with id "+id, "try slowhunch search")
	}
	curRev := frontmatter.Rev(e.Doc)
	if *revFlag != curRev {
		ce := errs.New(errs.ConflictRev, "stale rev for "+e.Path+": expected "+*revFlag+", current "+curRev,
			"re-read the file and retry with the current rev")
		ce.Extra = map[string]any{"current": string(e.Raw), "rev": curRev}
		return nil, ce
	}

	old := e.Doc
	d := old.Clone()
	if patch != nil {
		if err := applyPatch(d, patch); err != nil {
			return nil, err
		}
		if e.Type == "note" {
			oldEv, hadEv := old.Get("evidence")
			newEv, hasEv := d.Get("evidence")
			if hadEv != hasEv || (hadEv && !sameValue(oldEv, newEv)) {
				return nil, errs.New(errs.BadInput, "evidence is never rewritten; create a new atom instead", "")
			}
			if old.GetString("claim") != d.GetString("claim") {
				d.Body = vault.NoteBody(d.GetString("claim"), d.GetString("evidence"))
			}
		}
	} else {
		if e.Type != "topic" {
			return nil, badInput("append works on topics only")
		}
		d.Body = d.Body + "\n" + appendText
	}

	return commitDoc(env, v, cfg, "update", e, d, *revFlag)
}

// commitDoc validates and writes the changed copy d of entry e in one
// transaction (rev-checked against rev) and returns the update result.
func commitDoc(env *Env, v *vault.Vault, cfg vault.Config, command string, e *vault.Entry, d *frontmatter.Doc, rev string) (any, error) {
	id, old := e.ID, e.Doc
	curRev := frontmatter.Rev(old)
	result := func(rev, hash string) map[string]any {
		return map[string]any{"id": id, "rev": rev, "path": e.Path, "git": map[string]any{"commit": hash}}
	}
	if bytes.Equal(d.Bytes(), old.Bytes()) {
		return result(curRev, ""), nil
	}
	d.Set("updated", env.today())
	if err := vault.Validate(d, id); err != nil {
		return nil, err
	}

	txn, err := env.begin(v, cfg, command)
	if err != nil {
		return nil, err
	}
	newRev, err := txn.PutDoc(e.Path, d, rev)
	if err != nil {
		return nil, err
	}
	hash, err := txn.Commit([]string{id}, "updated: 1")
	if err != nil {
		return nil, err
	}
	return result(newRev, hash), nil
}

// applyPatch sets each patch key on d (create's value conversion); null deletes.
func applyPatch(d *frontmatter.Doc, patch map[string]any) error {
	sets := map[string]any{}
	for k, val := range patch {
		if k == "body" {
			if d.GetString("type") == "note" {
				return badInput("a note body is derived from claim and evidence")
			}
			s, ok := val.(string)
			if !ok {
				return badInput("body must be a string")
			}
			d.Body = s
			continue
		}
		if val == nil {
			d.Delete(k)
			continue
		}
		sets[k] = val
	}
	conv, err := docFromJSON(sets)
	if err != nil {
		return err
	}
	for _, f := range conv.Fields {
		d.Set(f.Key, f.Value)
	}
	return nil
}

func sameValue(a, b any) bool {
	x := &frontmatter.Doc{Fields: []frontmatter.Field{{Key: "v", Value: a}}}
	y := &frontmatter.Doc{Fields: []frontmatter.Field{{Key: "v", Value: b}}}
	return bytes.Equal(x.Bytes(), y.Bytes())
}
