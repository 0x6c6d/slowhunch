package intake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

func TestIntakeRead(t *testing.T) {
	dir := t.TempDir()
	w := func(n, c string) string {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if in, err := Read("hello world", nil); err != nil || in.Text != "hello world" {
		t.Fatal(in, err)
	}
	if in, err := Read("-", strings.NewReader("from stdin")); err != nil || in.Text != "from stdin" {
		t.Fatal(in, err)
	}
	if _, err := Read("-", strings.NewReader(strings.Repeat("a", 4*MaxChars+5))); errs.CodeOf(err) != errs.TooLong {
		t.Fatal(err)
	}
	for _, n := range []string{"a.txt", "b.MD"} {
		in, err := Read(w(n, "body"), nil)
		if err != nil || in.Text != "body" || in.Ext != strings.ToLower(filepath.Ext(n)) {
			t.Fatal(in, err)
		}
	}
	if _, err := Read(w("p.pdf", "%PDF"), nil); errs.CodeOf(err) != errs.Unsupported {
		t.Fatal(err)
	}
	if _, err := Read(filepath.Join(dir, "missing.pdf"), nil); errs.CodeOf(err) != errs.Unsupported {
		t.Fatal(err)
	}
	if _, err := Read(dir, nil); errs.CodeOf(err) != errs.BadInput {
		t.Fatal(err)
	}
	if _, err := Read(w("bad.txt", "\xff\xfe"), nil); errs.CodeOf(err) != errs.BadInput {
		t.Fatal(err)
	}
	if _, err := Read("-", strings.NewReader("\xff")); errs.CodeOf(err) != errs.BadInput {
		t.Fatal(err)
	}
	if in, err := Read("https://example.com/x", nil); err != nil || in.Text != "https://example.com/x" {
		t.Fatal(in, err)
	}
}

func TestIntakeCheck(t *testing.T) {
	cases := []struct {
		name                 string
		text, medium, ti, au string
		code                 string
		hint                 string
	}{
		{"max ascii", strings.Repeat("a", 12000), "article", "", "", "", ""},
		{"max multibyte", strings.Repeat("é", 12000), "article", "", "", "", ""},
		{"too long", strings.Repeat("a", 12001), "article", "", "", errs.TooLong, ""},
		{"empty", "", "article", "", "", errs.BadInput, ""},
		{"blank", " \n ", "article", "", "", errs.BadInput, ""},
		{"film no title", "a scene", "film", "", "", errs.BadInput, ""},
		{"film ok", "a scene", "film", "T", "", "", ""},
		{"art no text", "", "art", "T", "", errs.Unsupported, ArtHint},
		{"quote no author", "words", "quote", "", "", errs.BadInput, ""},
		{"quote ok", "words", "quote", "", "A", "", ""},
		{"poem no title", "words", "poem", "", "", errs.BadInput, ""},
		{"unknown medium", "words", "zzz", "", "", errs.BadInput, ""},
	}
	for _, c := range cases {
		err := Check(Input{Text: c.text}, c.medium, c.ti, c.au)
		if errs.CodeOf(err) != c.code {
			t.Errorf("%s: got %v want %q", c.name, err, c.code)
		}
		if c.hint != "" {
			if e, ok := err.(*errs.Error); !ok || e.Hint != c.hint {
				t.Errorf("%s: hint mismatch", c.name)
			}
		}
	}
}

func TestInferMedium(t *testing.T) {
	if m := InferMedium(Input{Text: strings.Repeat("a", 1001)}); m != "article" {
		t.Fatal(m)
	}
	if m := InferMedium(Input{Text: strings.Repeat("é", 1000)}); m != "other" {
		t.Fatal(m)
	}
	if m := InferMedium(Input{Text: "a\nb\nc"}); m != "other" {
		t.Fatal(m)
	}
}
