// Package vault models a knowledge base: a directory tree of markdown
// notes, the links between them, and import/export operations.
package vault

import "time"

// Note is a single markdown file in the vault.
type Note struct {
	// Path is relative to the vault root, always forward-slashed.
	Path    string
	Title   string   // first H1, frontmatter title, or filename stem
	Tags    []string // frontmatter tags plus inline #tags
	Links   []string // resolved relative paths of outgoing links
	ModTime time.Time
	Size    int64
}

// Base is a knowledge base within the vault. By default the whole vault
// is one base; each top-level directory is also addressable as a base.
type Base struct {
	Name      string // "" means the whole vault
	Path      string // relative directory, "" for the root base
	NoteCount int
}

// Node is a note in the link graph.
type Node struct {
	ID     int
	Path   string
	Title  string
	Degree int
	Base   string // top-level directory, "" for root-level notes
}

// Edge is a directed link between two nodes, by Node.ID.
type Edge struct {
	From, To int
}

// Graph is the resolved link graph of a vault (or one base of it).
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// Vault is an open knowledge base rooted at a directory.
type Vault struct {
	Root  string
	Notes map[string]*Note // keyed by Note.Path
}
