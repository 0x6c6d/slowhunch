package vault

import (
	"regexp"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

var linkRe = regexp.MustCompile(`\[\[([a-z0-9]+(?:-[a-z0-9]+)*)\]\]`)

// Links returns the ids a document links to, deduplicated in first-seen order.
func Links(d *frontmatter.Doc) []string {
	var out []string
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, m := range linkRe.FindAllStringSubmatch(d.Body, -1) {
		add(m[1])
	}
	for _, k := range []string{"source", "translates"} {
		add(d.GetString(k))
	}
	for _, k := range []string{"bears_on", "links", "unlocks", "rereads", "atoms", "items"} {
		for _, id := range d.GetList(k) {
			add(id)
		}
	}
	if d.GetString("type") == "hunch" || d.GetMaps("evidence") != nil {
		for _, m := range d.GetMaps("evidence") {
			for _, x := range linkRe.FindAllStringSubmatch(m["note"], -1) {
				add(x[1])
			}
		}
	}
	return out
}
