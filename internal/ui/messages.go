package ui

// Messages exchanged between views and the app shell. Views never talk
// to each other directly: they emit one of these and the shell routes.

// OpenNoteMsg asks the shell to show a note in the read-mode viewer.
type OpenNoteMsg struct{ Path string }

// EditExternalMsg asks the shell to suspend the TUI and open the note
// in the configured external editor, then reload it.
type EditExternalMsg struct{ Path string }

// BackMsg asks the shell to close the current view (":q"). From a note
// it returns to whatever is beneath it (another note or the graph);
// from the graph it quits the app.
type BackMsg struct{}

// NavBackMsg steps back one note in the link trail (browser "back").
// Unlike BackMsg it never falls through to the graph: at the origin
// note it is a no-op, so esc can never accidentally leave read mode.
type NavBackMsg struct{}

// StatusMsg puts transient text in the status bar.
type StatusMsg struct{ Text string }

// VaultChangedMsg reports that notes changed on disk; the shell rescans
// and refreshes the graph.
type VaultChangedMsg struct{}

// SwitchBaseMsg switches the active knowledge base ("" = whole vault).
type SwitchBaseMsg struct{ Base string }

// RevealNoteMsg brings a note into view on the graph: the shell widens the
// active base if the note lives outside it, then the graph view flies the
// camera to it. Emitted by fuzzy search, which spans the whole vault.
type RevealNoteMsg struct{ Path string }

// SearchEntry is one row in the graph's fuzzy "go to" index: a note
// (IsFolder false) reachable by title or path, or a folder that can be
// scoped to as a base. The shell supplies the whole-vault index so search
// is never limited to the active base.
type SearchEntry struct {
	Path     string // note path, or folder path (both vault-relative)
	Title    string // note title; for a folder, its path
	Base     string // top-level base it belongs to ("" = vault root)
	IsFolder bool
}
