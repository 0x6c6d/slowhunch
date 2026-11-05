// Package intake reads and checks the text slowhunch is asked to judge.
package intake

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
	execprov "github.com/0x6c6d/slowhunch/internal/provider/exec"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

// Input is the text to process; Path/Ext are empty for argument or stdin text.
type Input struct {
	Text string
	Path string
	Ext  string
}

const MaxChars = 12000

const ArtHint = "describe or quote the image or scene; there is no vision or audio model"

var binaryExts = []string{".pdf", ".doc", ".docx", ".epub", ".png", ".jpg", ".jpeg", ".gif", ".mp3", ".mp4", ".wav", ".mov"}

func readLimited(r io.Reader) (string, error) {
	limit := int64(4*MaxChars + 4)
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return "", errs.New(errs.BadInput, "cannot read input: "+err.Error(), "")
	}
	if int64(len(b)) > limit {
		return "", errs.New(errs.TooLong, "input is too long", "shorten it to at most 12000 characters")
	}
	if !utf8.Valid(b) {
		return "", errs.New(errs.BadInput, "input is not valid UTF-8", "")
	}
	return string(b), nil
}

// PDFHint is the hint of every failed PDF extraction.
const PDFHint = "install pdftotext or set pdf.command"

// DefaultPDFTimeout bounds one run of the PDF command when ReadOptions.PDFTimeout is zero.
const DefaultPDFTimeout = 60 * time.Second

// pdfStdoutCap caps the bytes read from the PDF command's standard output.
const pdfStdoutCap = 1 << 20

// ReadOptions configure ReadWith. An empty PDFCommand leaves .pdf files UNSUPPORTED.
type ReadOptions struct {
	PDFCommand []string
	PDFTimeout time.Duration
}

// Read resolves arg to text: stdin for "-", a .txt/.md file, or the argument itself.
func Read(arg string, stdin io.Reader) (Input, error) {
	return ReadWith(arg, stdin, ReadOptions{})
}

// readPDF runs the configured PDF command on the file at arg; its stdout is the text.
// The command is argv only (no shell) and every element exactly "{file}" becomes the
// cleaned absolute path of the file.
func readPDF(arg string, o ReadOptions) (Input, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return Input{}, errs.New(errs.Unsupported, "cannot resolve "+arg+": "+err.Error(), PDFHint)
	}
	argv := make([]string, len(o.PDFCommand))
	for i, a := range o.PDFCommand {
		if a == "{file}" {
			a = abs
		}
		argv[i] = a
	}
	timeout := o.PDFTimeout
	if timeout <= 0 {
		timeout = DefaultPDFTimeout
	}
	out, err := execprov.RunArgv(context.Background(), argv, nil, nil, timeout, pdfStdoutCap)
	if err != nil {
		msg := "pdf.command failed: " + err.Error()
		if execprov.IsNotFound(err) {
			msg = "pdf.command program not found: " + argv[0]
		}
		return Input{}, errs.New(errs.Unsupported, msg, PDFHint)
	}
	if !utf8.Valid(out) {
		return Input{}, errs.New(errs.BadInput, "pdf.command output is not valid UTF-8", PDFHint)
	}
	text := string(out)
	if utf8.RuneCountInString(text) > MaxChars {
		return Input{}, errs.New(errs.TooLong, "text extracted from "+arg+" is longer than 12000 characters", "shorten it; slowhunch never truncates")
	}
	if strings.TrimSpace(text) == "" {
		return Input{}, errs.New(errs.Unsupported, "pdf.command printed no text for "+arg, PDFHint)
	}
	return Input{Text: text, Path: arg, Ext: ".pdf"}, nil
}

// ReadWith is Read with options; see ReadOptions.
func ReadWith(arg string, stdin io.Reader, o ReadOptions) (Input, error) {
	if arg == "-" {
		if stdin == nil {
			stdin = strings.NewReader("")
		}
		t, err := readLimited(stdin)
		return Input{Text: t}, err
	}
	if st, err := os.Stat(arg); err == nil {
		if st.IsDir() {
			return Input{}, errs.New(errs.BadInput, arg+" is a directory", "pass a .txt or .md file, or the text itself")
		}
		ext := strings.ToLower(filepath.Ext(arg))
		if ext == ".pdf" && len(o.PDFCommand) > 0 {
			return readPDF(arg, o)
		}
		if ext != ".txt" && ext != ".md" {
			return Input{}, errs.New(errs.Unsupported, "only .txt and .md files are read", "convert the file to text first")
		}
		f, err := os.Open(arg)
		if err != nil {
			return Input{}, errs.New(errs.BadInput, "cannot read "+arg+": "+err.Error(), "")
		}
		defer f.Close()
		t, err := readLimited(f)
		if err != nil {
			return Input{}, err
		}
		return Input{Text: t, Path: arg, Ext: ext}, nil
	}
	if !strings.ContainsAny(arg, " \t\r\n") {
		ext := strings.ToLower(filepath.Ext(arg))
		for _, b := range binaryExts {
			if ext == b {
				return Input{}, errs.New(errs.Unsupported, "only .txt and .md files are read", "convert the file to text first")
			}
		}
	}
	if !utf8.ValidString(arg) {
		return Input{}, errs.New(errs.BadInput, "input is not valid UTF-8", "")
	}
	return Input{Text: arg}, nil
}

// Check validates the input against the medium and metadata.
func Check(in Input, medium, title, author string) error {
	known := false
	for _, m := range vault.Mediums {
		if m == medium {
			known = true
		}
	}
	if !known {
		return errs.New(errs.BadInput, "unknown medium "+medium, "")
	}
	empty := strings.TrimSpace(in.Text) == ""
	if empty && (medium == "art" || medium == "film") {
		return errs.New(errs.Unsupported, "no text to read for medium "+medium, ArtHint)
	}
	if empty {
		return errs.New(errs.BadInput, "input is empty", "")
	}
	if utf8.RuneCountInString(in.Text) > MaxChars {
		return errs.New(errs.TooLong, "input is longer than 12000 characters", "shorten it; slowhunch never truncates")
	}
	if (medium == "poem" || medium == "lyric" || medium == "film") && strings.TrimSpace(title) == "" {
		return errs.New(errs.BadInput, "medium "+medium+" needs a title", "pass -title")
	}
	if medium == "quote" && strings.TrimSpace(author) == "" {
		return errs.New(errs.BadInput, "a quote needs an author", "pass -author")
	}
	return nil
}

// InferMedium guesses a medium from length only; text from a PDF is an article.
func InferMedium(in Input) string {
	if in.Ext == ".pdf" {
		return "article"
	}
	if utf8.RuneCountInString(in.Text) > 1000 {
		return "article"
	}
	return "other"
}
