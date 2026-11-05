package cli

import (
	"strings"
	"unicode/utf8"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/frontmatter"
	"github.com/0x6c6d/slowhunch/internal/intake"
	"github.com/0x6c6d/slowhunch/internal/search"
)

func init() {
	register("keep", "append a dated note linking ids to a topic", cmdKeep)
}

const keepMaxChars = 2000

func cmdKeep(env *Env, args []string) (any, error) {
	fs := newFlagSet("keep")
	idsFlag := fs.String("ids", "", "comma-separated ids to link")
	topicFlag := fs.String("topic", "", "topic id (default: the nearest topic)")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	usage := `usage: slowhunch keep "<text>" --ids a,b [--topic ID]`
	if len(pos) != 1 {
		return nil, errs.New(errs.BadInput, "keep needs exactly one text", usage)
	}
	text := strings.TrimSpace(pos[0])
	if text == "" {
		return nil, errs.New(errs.BadInput, "keep text must not be empty", usage)
	}
	if utf8.RuneCountInString(text) > keepMaxChars {
		return nil, errs.New(errs.TooLong, "keep text is longer than 2000 characters", "shorten it")
	}
	if intake.ContainsSecret(text) {
		return nil, errs.New(errs.BadInput, "keep text looks like it contains a secret", "remove it and retry")
	}
	var ids []string
	for _, s := range strings.Split(*idsFlag, ",") {
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	if len(ids) == 0 {
		return nil, errs.New(errs.BadInput, "keep needs --ids", usage)
	}
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := ix.Get(id); !ok {
			return nil, errs.New(errs.NotFound, "no entry with id "+id, "try slowhunch search")
		}
	}
	topics := ix.OfType("topic")
	topicID := *topicFlag
	if topicID != "" {
		if e, ok := ix.Get(topicID); !ok || e.Type != "topic" {
			return nil, errs.New(errs.NotFound, "no topic with id "+topicID, "try slowhunch list topic")
		}
	} else {
		if len(topics) == 0 {
			return nil, errs.New(errs.NotFound, "no topics in the vault", "create a topic first")
		}
		var docs []search.Doc
		for _, t := range topics {
			docs = append(docs, search.Doc{ID: t.ID, Text: t.Doc.GetString("title") + " " + t.Doc.Body})
		}
		sx := search.Build(docs)
		q := sx.Query(text)
		best, bestScore := "", -1.0
		for _, t := range topics {
			sc := search.Cosine(q, sx.DocVector(t.ID))
			if sc > bestScore || (sc == bestScore && t.ID < best) {
				best, bestScore = t.ID, sc
			}
		}
		topicID = best
	}
	e, _ := ix.Get(topicID)

	links := make([]string, len(ids))
	for i, id := range ids {
		links[i] = "[[" + id + "]]"
	}
	d := e.Doc.Clone()
	d.Body = d.Body + "\n### " + env.today() + "\n" + text + "\n" + strings.Join(links, " ")
	return commitDoc(env, v, cfg, "keep", e, d, frontmatter.Rev(e.Doc))
}
