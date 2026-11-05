package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/frontmatter"
)

// Entry is one indexed vault file. Path is vault-relative.
type Entry struct {
	ID, Type, Path string
	Doc            *frontmatter.Doc
	Raw            []byte
}

// LoadError describes a file that could not be indexed.
type LoadError struct {
	Path string
	Err  error
}

// Index is an in-memory view of the vault, rebuilt on every command.
type Index struct {
	byID map[string]*Entry
	ids  []string
	out  map[string][]string
	in   map[string][]string
}

var indexFolders = []string{"notes", "sources", "hunches", "proposals", "topics"}

// LoadIndex reads all entries; per-file problems come back as LoadErrors.
func (v *Vault) LoadIndex() (*Index, []LoadError, error) {
	ix := &Index{byID: map[string]*Entry{}, out: map[string][]string{}, in: map[string][]string{}}
	var lerrs []LoadError
	for _, folder := range indexFolders {
		des, err := os.ReadDir(filepath.Join(v.Root, folder))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		for _, de := range des { // ReadDir is sorted by name
			name := de.Name()
			if de.IsDir() || !strings.HasSuffix(name, ".md") {
				continue
			}
			rel := folder + "/" + name
			fail := func(err error) { lerrs = append(lerrs, LoadError{Path: rel, Err: err}) }
			raw, err := os.ReadFile(filepath.Join(v.Root, folder, name))
			if err != nil {
				fail(err)
				continue
			}
			doc, err := frontmatter.Parse(raw)
			if err != nil {
				fail(err)
				continue
			}
			id := strings.TrimSuffix(name, ".md")
			typ := doc.GetString("type")
			if Folder(typ) != folder {
				fail(fmt.Errorf("type %q does not belong in %s/", typ, folder))
				continue
			}
			if doc.GetString("id") != id {
				fail(fmt.Errorf("id %q does not match file name %q", doc.GetString("id"), id))
				continue
			}
			if prev, dup := ix.byID[id]; dup {
				fail(fmt.Errorf("duplicate id %q (also %s)", id, prev.Path))
				continue
			}
			ix.byID[id] = &Entry{ID: id, Type: typ, Path: rel, Doc: doc, Raw: raw}
			ix.ids = append(ix.ids, id)
		}
	}
	sort.Strings(ix.ids)
	for _, id := range ix.ids {
		links := Links(ix.byID[id].Doc)
		ix.out[id] = links
		for _, to := range links {
			if _, ok := ix.byID[to]; ok {
				ix.in[to] = append(ix.in[to], id)
			}
		}
	}
	return ix, lerrs, nil
}

// Get returns the entry with the id.
func (ix *Index) Get(id string) (*Entry, bool) {
	e, ok := ix.byID[id]
	return e, ok
}

// All returns every entry sorted by id.
func (ix *Index) All() []*Entry {
	r := make([]*Entry, 0, len(ix.ids))
	for _, id := range ix.ids {
		r = append(r, ix.byID[id])
	}
	return r
}

// OfType returns entries of a type sorted by id.
func (ix *Index) OfType(typ string) []*Entry {
	var r []*Entry
	for _, id := range ix.ids {
		if e := ix.byID[id]; e.Type == typ {
			r = append(r, e)
		}
	}
	return r
}

// Outgoing returns the ids the entry links to (possibly unknown ones).
func (ix *Index) Outgoing(id string) []string { return ix.out[id] }

// Backlinks returns the sorted ids of entries linking to id.
func (ix *Index) Backlinks(id string) []string {
	r := append([]string(nil), ix.in[id]...)
	sort.Strings(r)
	return r
}

// Neighbourhood walks outgoing and incoming links up to depth.
func (ix *Index) Neighbourhood(id string, depth int) (nodes []string, edges [][2]string) {
	seen := map[string]bool{id: true}
	frontier := []string{id}
	edgeSet := map[[2]string]bool{}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, n := range frontier {
			visit := func(m string, e [2]string) {
				if _, ok := ix.byID[m]; !ok {
					return
				}
				edgeSet[e] = true
				if !seen[m] {
					seen[m] = true
					next = append(next, m)
				}
			}
			for _, m := range ix.out[n] {
				visit(m, [2]string{n, m})
			}
			for _, m := range ix.in[n] {
				visit(m, [2]string{m, n})
			}
		}
		frontier = next
	}
	for n := range seen {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for e := range edgeSet {
		edges = append(edges, e)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})
	return
}

// MediumOf returns a source's medium, or for a note its source's medium.
func (ix *Index) MediumOf(id string) string {
	e, ok := ix.byID[id]
	if !ok {
		return ""
	}
	switch e.Type {
	case "source":
		return e.Doc.GetString("medium")
	case "note":
		if s, ok := ix.byID[e.Doc.GetString("source")]; ok && s.Type == "source" {
			return s.Doc.GetString("medium")
		}
	}
	return ""
}
