// Package intake reads and checks the text slowhunch is asked to judge.
package intake

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
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

// Read resolves arg to text: stdin for "-", a .txt/.md file, or the argument itself.
func Read(arg string, stdin io.Reader) (Input, error) {
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

// InferMedium guesses a medium from length only.
func InferMedium(in Input) string {
	if utf8.RuneCountInString(in.Text) > 1000 {
		return "article"
	}
	return "other"
}
