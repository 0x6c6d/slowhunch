package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

func badInput(msg string) error { return errs.New(errs.BadInput, msg, "") }

func scalarString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case json.Number:
		return x.String(), true
	case int:
		return strconv.Itoa(x), true
	}
	return "", false
}

// docFromJSON converts decoded JSON fields into a Doc (keys sorted).
func docFromJSON(fields map[string]any) (*frontmatter.Doc, error) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	d := &frontmatter.Doc{}
	for _, k := range keys {
		v := fields[k]
		if s, ok := scalarString(v); ok {
			d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: s})
			continue
		}
		switch x := v.(type) {
		case []string:
			d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: append([]string{}, x...)})
		case []map[string]string:
			d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: x})
		case []any:
			if len(x) > 0 {
				if _, isObj := x[0].(map[string]any); isObj {
					ms := make([]map[string]string, 0, len(x))
					for _, e := range x {
						o, ok := e.(map[string]any)
						if !ok {
							return nil, badInput(fmt.Sprintf("field %q: mixed list", k))
						}
						m := map[string]string{}
						for mk, mv := range o {
							s, ok := scalarString(mv)
							if !ok {
								return nil, badInput(fmt.Sprintf("field %q: nested values are not supported", k))
							}
							m[mk] = s
						}
						ms = append(ms, m)
					}
					d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: ms})
					continue
				}
			}
			ss := make([]string, 0, len(x))
			for _, e := range x {
				s, ok := scalarString(e)
				if !ok {
					return nil, badInput(fmt.Sprintf("field %q: list items must be scalars or objects", k))
				}
				ss = append(ss, s)
			}
			d.Fields = append(d.Fields, frontmatter.Field{Key: k, Value: ss})
		default:
			return nil, badInput(fmt.Sprintf("field %q: unsupported value (null or nested object)", k))
		}
	}
	return d, nil
}

// docToJSON renders the fields of a Doc as JSON-ready values.
func docToJSON(d *frontmatter.Doc) map[string]any {
	out := map[string]any{}
	for _, f := range d.Fields {
		switch v := f.Value.(type) {
		case []string:
			if v == nil {
				v = []string{}
			}
			out[f.Key] = v
		case []map[string]string:
			if v == nil {
				v = []map[string]string{}
			}
			out[f.Key] = v
		default:
			out[f.Key] = fmt.Sprint(v)
		}
	}
	return out
}
