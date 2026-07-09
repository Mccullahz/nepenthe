// Package keymap defines every rebindable action in nepenthe and the
// mapping from terminal keys to those actions. Defaults are vim-flavored;
// both the Lua config and the in-app settings view rewrite this table.
package keymap

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Action is a named, rebindable operation. The string value is the
// identifier used in Lua configs, e.g. keymap.set("orbit_left", "h").
type Action string

const (
	// Global
	Quit    Action = "quit"
	Help    Action = "help"
	Back    Action = "back"
	Command Action = "command"

	// Graph navigation
	OrbitLeft    Action = "orbit_left"
	OrbitRight   Action = "orbit_right"
	OrbitUp      Action = "orbit_up"
	OrbitDown    Action = "orbit_down"
	ZoomIn       Action = "zoom_in"
	ZoomOut      Action = "zoom_out"
	NextNode     Action = "next_node"
	PrevNode     Action = "prev_node"
	OpenNode     Action = "open_node"
	ResetView    Action = "reset_view"
	ToggleLabels Action = "toggle_labels"
	ToggleFocus  Action = "toggle_focus"
	Search       Action = "search"
	SwitchBase   Action = "switch_base"

	// Note viewing
	ScrollUp   Action = "scroll_up"
	ScrollDown Action = "scroll_down"
	PageUp     Action = "page_up"
	PageDown   Action = "page_down"
	GotoTop    Action = "goto_top"
	GotoBottom Action = "goto_bottom"
	NextLink   Action = "next_link"
	PrevLink   Action = "prev_link"
	FollowLink Action = "follow_link"
	// LinkBack steps back through the trail of followed links (browser
	// "back"); it never leaves read mode. Use :q to close the note.
	LinkBack Action = "link_back"
	// Edit opens the current note in the external $EDITOR.
	Edit Action = "edit"
)

// Actions returns every rebindable action, for validation in the Lua
// config and listing in the settings UI.
func Actions() []Action {
	return []Action{
		Quit, Help, Back, Command,
		OrbitLeft, OrbitRight, OrbitUp, OrbitDown,
		ZoomIn, ZoomOut, NextNode, PrevNode, OpenNode,
		ResetView, ToggleLabels, ToggleFocus, Search, SwitchBase,
		ScrollUp, ScrollDown, PageUp, PageDown, GotoTop, GotoBottom,
		NextLink, PrevLink, FollowLink, LinkBack, Edit,
	}
}

// Keymap maps actions to the key strings that trigger them, in
// tea.KeyMsg.String() form (e.g. "h", "ctrl+d", "shift+tab", "enter").
type Keymap map[Action][]string

// Default returns the stock vim-flavored bindings.
func Default() Keymap {
	return Keymap{
		// Quitting is vim-style: :q from the graph. ctrl+c stays as the
		// emergency hard-exit. Back has no default key (use :q); it remains
		// rebindable for anyone who wants a single-key close.
		Quit:    {"ctrl+c"},
		Help:    {"?"},
		Back:    {},
		Command: {":"},

		OrbitLeft:    {"h", "left"},
		OrbitRight:   {"l", "right"},
		OrbitUp:      {"k", "up"},
		OrbitDown:    {"j", "down"},
		ZoomIn:       {"K", "+"},
		ZoomOut:      {"J", "-"},
		NextNode:     {"n", "tab"},
		PrevNode:     {"N", "shift+tab"},
		OpenNode:     {"enter"},
		ResetView:    {"0"},
		ToggleLabels: {"L"},
		ToggleFocus:  {"f"},
		Search:       {"/"},
		SwitchBase:   {"b"},

		ScrollUp:   {"k", "up"},
		ScrollDown: {"j", "down"},
		PageUp:     {"ctrl+u", "pgup"},
		PageDown:   {"ctrl+d", "pgdown"},
		GotoTop:    {"g"},
		GotoBottom: {"G"},
		NextLink:   {"tab", "n"},
		PrevLink:   {"shift+tab", "N"},
		FollowLink: {"enter"},
		LinkBack:   {"esc"},
		Edit:       {"i", "a"},
	}
}

// Matches reports whether the key message triggers the action.
func (k Keymap) Matches(msg tea.KeyMsg, a Action) bool {
	s := msg.String()
	for _, key := range k[a] {
		if s == key {
			return true
		}
	}
	return false
}

// Set replaces the bindings for an action.
func (k Keymap) Set(a Action, keys ...string) {
	k[a] = keys
}

// Hint returns a short display string for the action's bindings,
// e.g. "h/left", for help screens and status bars.
func (k Keymap) Hint(a Action) string {
	return strings.Join(k[a], "/")
}
