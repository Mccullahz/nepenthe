package ui

import tea "github.com/charmbracelet/bubbletea"

// View is one full-screen mode of the app (graph, note viewer, editor).
// The shell owns sizing and routing; views own their interior state.
type View interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (View, tea.Cmd)
	View() string
	SetSize(width, height int)
}
