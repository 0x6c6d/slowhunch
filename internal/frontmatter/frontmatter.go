// Package frontmatter parses and writes the strict YAML-like frontmatter
// subset used by vault files: "---\n<fields>---\n<body>".
package frontmatter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

// Field is one top-level key. Value is string, []string or []map[string]string.
type Field struct {
	Key   string
	Value any
}

// Doc is a parsed vault file.
type Doc struct {
	Fields []Field
	Body   string
}

var (
	keyRe   = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	plainRe = regexp.MustCompile(`^[A-Za-z0-9_./@+-][A-Za-z0-9 _./@+()-]*$`)
)

func bad(line int, format string, a ...any) error {
	return errs.New(errs.BadInput, fmt.Sprintf("frontmatter line %d: ", line)+fmt.Sprintf(format, a...),
		"use the canonical frontmatter subset (plain or JSON-quoted scalars, lists, lists of maps)")
}

// Parse parses b. Errors are *errs.Error with code BAD_INPUT naming the line.
func Parse(b []byte) (*Doc, error) {
	if bytes.IndexByte(b, '\r') >= 0 {
		line := 1 + bytes.Count(b[:bytes.IndexByte(b, '\r')], []byte("\n"))
		return nil, bad(line, "CR characters are not allowed")
	}
	if !bytes.HasPrefix(b, []byte("---\n")) {
		return nil, bad(1, "missing opening ---")
	}
	rest := string(b[4:])
	var lines []string
	body := ""
	closed := false
	for len(rest) > 0 {
		var ln string
		if i := strings.IndexByte(rest, '\n'); i >= 0 {
			ln, rest = rest[:i], rest[i+1:]
		} else {
			ln, rest = rest, ""
		}
		if ln == "---" {
			closed = true
			body = rest
			break
		}
		lines = append(lines, ln)
	}
	if !closed {
		return nil, bad(len(lines)+1, "missing closing ---")
	}
	d := &Doc{Body: body}
	seen := map[string]bool{}
	i := 0
	for i < len(lines) {
		lineNo := i + 2
		ln := lines[i]
		if strings.ContainsRune(ln, '\t') {
			return nil, bad(lineNo, "tabs are not allowed")
		}
		if ln == "" || ln[0] == ' ' {
			return nil, bad(lineNo, "bad indentation or empty line")
		}
		c := strings.IndexByte(ln, ':')
		if c < 0 {
			return nil, bad(lineNo, "expected key: value")
		}
		key := ln[:c]
		if !keyRe.MatchString(key) {
			return nil, bad(lineNo, "invalid key %q", key)
		}
		if seen[key] {
			return nil, bad(lineNo, "duplicate key %q", key)
		}
		seen[key] = true
		after := ln[c+1:]
		i++
		if after == "" {
			v, n, err := parseList(lines, i)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, bad(lineNo, "key %q has no items", key)
			}
			i += n
			d.Fields = append(d.Fields, Field{key, v})
			continue
		}
		if !strings.HasPrefix(after, " ") {
			return nil, bad(lineNo, "expected a space after ':'")
		}
		val := after[1:]
		if val == "[]" {
			d.Fields = append(d.Fields, Field{key, []string{}})
			continue
		}
		s, err := parseScalar(val, lineNo)
		if err != nil {
			return nil, err
		}
		d.Fields = append(d.Fields, Field{key, s})
	}
	return d, nil
}

// parseList reads list items starting at lines[start]; returns value and lines consumed.
func parseList(lines []string, start int) (any, int, error) {
	var scalars []string
	var maps []map[string]string
	i := start
	for i < len(lines) {
		ln := lines[i]
		lineNo := i + 2
		if !strings.HasPrefix(ln, " ") {
			break
		}
		if strings.ContainsRune(ln, '\t') {
			return nil, 0, bad(lineNo, "tabs are not allowed")
		}
		if !strings.HasPrefix(ln, "  - ") || strings.HasPrefix(ln, "  -  ") {
			return nil, 0, bad(lineNo, "bad indentation: expected '  - <item>'")
		}
		content := ln[4:]
		if k, v, ok := splitMapEntry(content); ok {
			if scalars != nil {
				return nil, 0, bad(lineNo, "mixed list of scalars and maps")
			}
			m := map[string]string{}
			if err := addMapEntry(m, k, v, lineNo); err != nil {
				return nil, 0, err
			}
			i++
			for i < len(lines) && strings.HasPrefix(lines[i], "    ") {
				cl := lines[i]
				clNo := i + 2
				if strings.ContainsRune(cl, '\t') {
					return nil, 0, bad(clNo, "tabs are not allowed")
				}
				ck, cv, ok := splitMapEntry(cl[4:])
				if !ok {
					return nil, 0, bad(clNo, "expected 'k: v' continuation or nesting too deep")
				}
				if err := addMapEntry(m, ck, cv, clNo); err != nil {
					return nil, 0, err
				}
				i++
			}
			maps = append(maps, m)
			continue
		}
		if maps != nil {
			return nil, 0, bad(lineNo, "mixed list of scalars and maps")
		}
		if content == "[]" {
			return nil, 0, bad(lineNo, "nested lists are not allowed")
		}
		s, err := parseScalar(content, lineNo)
		if err != nil {
			return nil, 0, err
		}
		scalars = append(scalars, s)
		i++
	}
	n := i - start
	if maps != nil {
		return maps, n, nil
	}
	return scalars, n, nil
}

// splitMapEntry reports whether content looks like "k: v" or "k:" with a valid key.
func splitMapEntry(content string) (string, *string, bool) {
	c := strings.IndexByte(content, ':')
	if c <= 0 || !keyRe.MatchString(content[:c]) {
		return "", nil, false
	}
	after := content[c+1:]
	if after == "" {
		return content[:c], nil, true
	}
	if !strings.HasPrefix(after, " ") {
		return "", nil, false
	}
	v := after[1:]
	return content[:c], &v, true
}

func addMapEntry(m map[string]string, k string, v *string, lineNo int) error {
	if v == nil {
		return bad(lineNo, "nested structure under %q is too deep", k)
	}
	if _, dup := m[k]; dup {
		return bad(lineNo, "duplicate key %q in list item", k)
	}
	s, err := parseScalar(*v, lineNo)
	if err != nil {
		return err
	}
	m[k] = s
	return nil
}

func parseScalar(v string, lineNo int) (string, error) {
	if strings.HasPrefix(v, `"`) {
		if len(v) < 2 || !strings.HasSuffix(v, `"`) {
			return "", bad(lineNo, "unterminated quoted string")
		}
		var s string
		if err := json.Unmarshal([]byte(v), &s); err != nil {
			return "", bad(lineNo, "invalid quoted string: %v", err)
		}
		return s, nil
	}
	if v == "" {
		return "", bad(lineNo, `empty value: write ""`)
	}
	if !plainRe.MatchString(v) || strings.HasSuffix(v, " ") {
		return "", bad(lineNo, "unsupported value %q (quote it as a JSON string)", v)
	}
	return v, nil
}

func formatScalar(s string) string {
	if plainRe.MatchString(s) && !strings.HasSuffix(s, " ") {
		return s
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}

// canonicalBody trims trailing whitespace and appends one newline when non-empty.
func canonicalBody(b string) string {
	b = strings.TrimRightFunc(b, unicode.IsSpace)
	if b == "" {
		return ""
	}
	return b + "\n"
}

// Bytes returns the canonical form of d.
func (d *Doc) Bytes() []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, f := range d.Fields {
		switch v := f.Value.(type) {
		case []string:
			if len(v) == 0 {
				sb.WriteString(f.Key + ": []\n")
				continue
			}
			sb.WriteString(f.Key + ":\n")
			for _, s := range v {
				sb.WriteString("  - " + formatScalar(s) + "\n")
			}
		case []map[string]string:
			if len(v) == 0 {
				sb.WriteString(f.Key + ": []\n")
				continue
			}
			sb.WriteString(f.Key + ":\n")
			for _, m := range v {
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for j, k := range keys {
					prefix := "    "
					if j == 0 {
						prefix = "  - "
					}
					sb.WriteString(prefix + k + ": " + formatScalar(m[k]) + "\n")
				}
			}
		case string:
			sb.WriteString(f.Key + ": " + formatScalar(v) + "\n")
		default:
			sb.WriteString(f.Key + ": " + formatScalar(fmt.Sprint(v)) + "\n")
		}
	}
	sb.WriteString("---\n")
	sb.WriteString(canonicalBody(d.Body))
	return []byte(sb.String())
}

// Get returns the raw value of key.
func (d *Doc) Get(key string) (any, bool) {
	for _, f := range d.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

// GetString returns the scalar value of key, or "" if absent or not a scalar.
func (d *Doc) GetString(key string) string {
	v, _ := d.Get(key)
	s, _ := v.(string)
	return s
}

// GetList returns the list value of key, or nil if absent or not a list.
func (d *Doc) GetList(key string) []string {
	v, _ := d.Get(key)
	switch l := v.(type) {
	case []string:
		if l == nil {
			return []string{}
		}
		return l
	case []map[string]string:
		if len(l) == 0 {
			return []string{}
		}
	}
	return nil
}

// GetMaps returns the list-of-maps value of key, or nil if absent or not one.
func (d *Doc) GetMaps(key string) []map[string]string {
	v, _ := d.Get(key)
	switch l := v.(type) {
	case []map[string]string:
		if l == nil {
			return []map[string]string{}
		}
		return l
	case []string:
		if len(l) == 0 {
			return []map[string]string{}
		}
	}
	return nil
}

// Set replaces key in place, or appends it at the end.
func (d *Doc) Set(key string, v any) {
	for i := range d.Fields {
		if d.Fields[i].Key == key {
			d.Fields[i].Value = v
			return
		}
	}
	d.Fields = append(d.Fields, Field{key, v})
}

// Delete removes key if present.
func (d *Doc) Delete(key string) {
	for i := range d.Fields {
		if d.Fields[i].Key == key {
			d.Fields = append(d.Fields[:i:i], d.Fields[i+1:]...)
			return
		}
	}
}

// Clone returns a deep copy of d.
func (d *Doc) Clone() *Doc {
	c := &Doc{Body: d.Body, Fields: make([]Field, len(d.Fields))}
	for i, f := range d.Fields {
		switch v := f.Value.(type) {
		case []string:
			if v != nil {
				f.Value = append([]string{}, v...)
			}
		case []map[string]string:
			if v != nil {
				ms := make([]map[string]string, len(v))
				for j, m := range v {
					mm := make(map[string]string, len(m))
					for k, x := range m {
						mm[k] = x
					}
					ms[j] = mm
				}
				f.Value = ms
			}
		}
		c.Fields[i] = f
	}
	return c
}

// Rev returns the first 6 hex chars of sha256 of the canonical bytes without "rev".
func Rev(d *Doc) string {
	c := d.Clone()
	c.Delete("rev")
	sum := sha256.Sum256(c.Bytes())
	return hex.EncodeToString(sum[:])[:6]
}

// LineOf returns the 1-based line of a top-level frontmatter key in b, 0 if absent.
func LineOf(b []byte, key string) int {
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		ln := lines[i]
		if ln == "---" {
			break
		}
		if strings.HasPrefix(ln, key+":") {
			return i + 1
		}
	}
	return 0
}
