package vault

import (
	"strconv"
	"strings"
)

// NoteBody renders the body of a note.
func NoteBody(claim, evidence string) string {
	lines := strings.Split(evidence, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + l
		}
	}
	return claim + "\n\n" + strings.Join(lines, "\n") + "\n"
}

// NumberedMedium reports whether inbox files of this medium carry line numbers.
func NumberedMedium(m string) bool {
	switch m {
	case "poem", "lyric", "prose", "quote", "film", "code":
		return true
	}
	return false
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// FormatInbox renders normalized text in stored inbox form.
func FormatInbox(normalized, medium string) []byte {
	if !NumberedMedium(medium) {
		return []byte(normalized + "\n")
	}
	var sb strings.Builder
	for i, l := range splitLines(normalized) {
		sb.WriteString(strconv.Itoa(i + 1))
		sb.WriteByte('\t')
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}

// InboxLines returns the content lines of a stored inbox file.
func InboxLines(stored []byte, medium string) []string {
	lines := splitLines(string(stored))
	if NumberedMedium(medium) {
		for i, l := range lines {
			if _, rest, ok := strings.Cut(l, "\t"); ok {
				lines[i] = rest
			}
		}
	}
	return lines
}

// InboxPlain returns the stored text without line numbers.
func InboxPlain(stored []byte, medium string) string {
	return strings.Join(InboxLines(stored, medium), "\n")
}
