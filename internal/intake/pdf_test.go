package intake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

const pdfMarker = "slowhunch-pdf-helper"

// init turns the test binary into a fake PDF extractor when re-exec'd with pdfMarker:
// argv is <exe> pdfMarker <mode> args...
func init() {
	if len(os.Args) < 3 || os.Args[1] != pdfMarker {
		return
	}
	switch os.Args[2] {
	case "echo":
		a, _ := json.Marshal(os.Args[3:])
		e, _ := json.Marshal(os.Environ())
		fmt.Printf("Extracted PDF text.\nARGS %s\nENV %s\n", a, e)
	case "fail":
		fmt.Fprintln(os.Stderr, "broken pdf")
		os.Exit(1)
	case "empty":
		fmt.Print("  \n\t\n")
	case "long":
		fmt.Print(strings.Repeat("a", MaxChars+1))
	}
	os.Exit(0)
}

func pdfHelper(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, pdfMarker, mode}, args...)
}

func writePDF(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPDFExtract(t *testing.T) {
	t.Setenv("SLOWHUNCH_LEAK", "leaked-value")
	p := writePDF(t, "a; echo $(id) `x` | y & z.pdf")
	// A relative, uncleaned path must arrive as the cleaned absolute path.
	dir, name := filepath.Split(p)
	t.Chdir(dir)
	arg := "./sub/../" + name
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	in, err := ReadWith(arg, nil, ReadOptions{PDFCommand: pdfHelper(t, "echo", "{file}", "--in={file}", "-")})
	if err != nil {
		t.Fatal(err)
	}
	if in.Path != arg || in.Ext != ".pdf" || !strings.HasPrefix(in.Text, "Extracted PDF text.\n") {
		t.Fatalf("input %+v", in)
	}
	if InferMedium(in) != "article" {
		t.Fatalf("medium %q", InferMedium(in))
	}
	var gotArgs, gotEnv []string
	for _, l := range strings.Split(in.Text, "\n") {
		if v, ok := strings.CutPrefix(l, "ARGS "); ok {
			if err := json.Unmarshal([]byte(v), &gotArgs); err != nil {
				t.Fatal(err)
			}
		}
		if v, ok := strings.CutPrefix(l, "ENV "); ok {
			if err := json.Unmarshal([]byte(v), &gotEnv); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := []string{p, "--in={file}", "-"}
	if fmt.Sprint(gotArgs) != fmt.Sprint(want) || len(gotArgs) != 3 {
		t.Fatalf("args %q, want %q", gotArgs, want)
	}
	if len(gotEnv) != 1 || !strings.HasPrefix(gotEnv[0], "PATH=") {
		t.Fatalf("env %q, want only PATH", gotEnv)
	}
	if strings.Contains(in.Text, "SLOWHUNCH_LEAK") || strings.Contains(in.Text, "leaked-value") {
		t.Fatal("environment leaked to the pdf command")
	}
	// Without a command, plain Read keeps .pdf UNSUPPORTED.
	if _, err := Read(p, nil); errs.CodeOf(err) != errs.Unsupported {
		t.Fatal(err)
	}
}

func TestPDFUnsupported(t *testing.T) {
	p := writePDF(t, "paper.pdf")
	cases := []struct {
		name string
		cmd  []string
		code string
		hint string
	}{
		{"missing program", []string{"slowhunch-no-such-pdf-tool", "{file}"}, errs.Unsupported, PDFHint},
		{"exit 1", pdfHelper(t, "fail", "{file}"), errs.Unsupported, PDFHint},
		{"empty output", pdfHelper(t, "empty", "{file}"), errs.Unsupported, PDFHint},
		{"too long", pdfHelper(t, "long", "{file}"), errs.TooLong, ""},
	}
	for _, c := range cases {
		in, err := ReadWith(p, nil, ReadOptions{PDFCommand: c.cmd})
		if errs.CodeOf(err) != c.code {
			t.Errorf("%s: got %+v %v, want %s", c.name, in, err, c.code)
			continue
		}
		e, ok := err.(*errs.Error)
		if !ok || e.Hint == "" || (c.hint != "" && e.Hint != c.hint) {
			t.Errorf("%s: hint %v", c.name, err)
		}
	}
}
