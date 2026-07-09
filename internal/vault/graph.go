package vault

import (
	"sort"
	"strings"
)

// Graph returns the link graph for one base ("" = whole vault). Only
// notes under the base, and edges between those notes, are included.
// Node ordering is deterministic (sorted by path).
func (v *Vault) Graph(base string) *Graph {
	paths := make([]string, 0, len(v.Notes))
	for p := range v.Notes {
		if inBase(p, base) {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	id := make(map[string]int, len(paths))
	g := &Graph{Nodes: make([]Node, 0, len(paths))}
	for i, p := range paths {
		id[p] = i
		g.Nodes = append(g.Nodes, Node{
			ID:    i,
			Path:  p,
			Title: v.Notes[p].Title,
			Base:  topLevelDir(p),
		})
	}

	for _, p := range paths {
		from := id[p]
		for _, to := range v.Notes[p].Links {
			toID, ok := id[to]
			if !ok {
				continue
			}
			g.Edges = append(g.Edges, Edge{From: from, To: toID})
			g.Nodes[from].Degree++
			g.Nodes[toID].Degree++
		}
	}
	return g
}

// Bases lists the whole vault plus each top-level directory, each with
// its own (recursive) note count.
func (v *Vault) Bases() []Base {
	counts := make(map[string]int)
	for p := range v.Notes {
		if dir := topLevelDir(p); dir != "" {
			counts[dir]++
		}
	}

	bases := []Base{{Name: "", Path: "", NoteCount: len(v.Notes)}}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		bases = append(bases, Base{Name: n, Path: n, NoteCount: counts[n]})
	}
	return bases
}

func inBase(p, base string) bool {
	return base == "" || strings.HasPrefix(p, base+"/")
}

// topLevelDir returns the first path segment of a vault-relative path,
// or "" if the note is at the vault root.
func topLevelDir(p string) string {
	if idx := strings.IndexByte(p, '/'); idx >= 0 {
		return p[:idx]
	}
	return ""
}
