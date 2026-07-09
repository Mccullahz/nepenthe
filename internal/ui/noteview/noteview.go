// Package noteview is the read mode for a note: glamour-rendered
// markdown in a scrolling viewport, in-document link navigation, and
// jumping-off points into the editors.
package noteview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/keymap"
	"github.com/mccullahz/nepenthe-cli/internal/ui"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

// Model is the note viewer.
type Model struct {
	cfg  *config.Config
	v    *vault.Vault
	path string

	vp            viewport.Model
	width, height int

	raw      string // raw markdown source
	rendered string // glamour output with sentinels, cached per size
	links    []link // navigable links in document order
	current  int    // selected link index, -1 when there are none

	linkStyle    lipgloss.Style
	curLinkStyle lipgloss.Style
}

// New builds a viewer for one note.
func New(cfg *config.Config, v *vault.Vault, path string) *Model {
	m := &Model{
		cfg:     cfg,
		v:       v,
		path:    path,
		vp:      viewport.New(0, 0),
		current: -1,
	}
	raw, err := v.Read(path)
	if err != nil {
		raw = "# error\n\n" + err.Error()
	}
	m.raw = stripFrontmatter(raw)
	m.buildStyles()
	m.links = extractLinks(m.raw)
	m.resolveLinks()
	if len(m.links) > 0 {
		m.current = 0
	}
	return m
}

// Path is the note being viewed, relative to the vault root.
func (m *Model) Path() string { return m.path }

func (m *Model) Init() tea.Cmd { return nil }

// SetSize fixes the viewer to width x height; the body takes all but the
// final footer row. Re-renders markdown to the new wrap width.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	bodyH := h - 1
	if bodyH < 0 {
		bodyH = 0
	}
	m.vp.Width = w
	m.vp.Height = bodyH
	m.render()
}

// render rebuilds the glamour output for the current width and refreshes
// the viewport content with link highlights applied.
func (m *Model) render() {
	if m.width <= 0 {
		return
	}
	wrap := m.width - 2
	if wrap < 1 {
		wrap = 1
	}
	transformed := transformSource(m.raw, m.links)
	out := transformed
	if r, err := newRenderer(m.cfg.Theme.GlamourStyle, wrap); err == nil {
		if s, err := r.Render(transformed); err == nil {
			out = s
		}
	}
	m.rendered = out
	m.refreshContent()
}

// refreshContent re-applies link highlighting to the cached render and
// pushes it into the viewport. Cheap enough to call on every selection
// change without re-invoking glamour.
func (m *Model) refreshContent() {
	m.vp.SetContent(m.applyHighlights(m.rendered))
}

func (m *Model) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	km := m.cfg.Keymap
	switch {
	case km.Matches(key, keymap.LinkBack):
		// esc steps back through the link trail; the shell no-ops it at
		// the origin note so it never drops us to the graph.
		return m, emit(ui.NavBackMsg{})
	case km.Matches(key, keymap.Edit):
		return m, emit(ui.EditExternalMsg{Path: m.path})
	case km.Matches(key, keymap.FollowLink):
		return m, m.followLink()
	case km.Matches(key, keymap.NextLink):
		m.cycleLink(1)
		return m, nil
	case km.Matches(key, keymap.PrevLink):
		m.cycleLink(-1)
		return m, nil
	case km.Matches(key, keymap.ScrollUp):
		m.vp.LineUp(1)
		return m, nil
	case km.Matches(key, keymap.ScrollDown):
		m.vp.LineDown(1)
		return m, nil
	case km.Matches(key, keymap.PageUp):
		m.vp.HalfPageUp()
		return m, nil
	case km.Matches(key, keymap.PageDown):
		m.vp.HalfPageDown()
		return m, nil
	case km.Matches(key, keymap.GotoTop):
		m.vp.GotoTop()
		return m, nil
	case km.Matches(key, keymap.GotoBottom):
		m.vp.GotoBottom()
		return m, nil
	}
	return m, nil
}

// cycleLink advances the selected link by dir (+1/-1), wrapping around.
func (m *Model) cycleLink(dir int) {
	n := len(m.links)
	if n == 0 {
		return
	}
	m.current = ((m.current+dir)%n + n) % n
	m.refreshContent()
}

// followLink opens the current link's target, or reports why it can't.
func (m *Model) followLink() tea.Cmd {
	if m.current < 0 || m.current >= len(m.links) {
		return nil
	}
	l := m.links[m.current]
	if l.resolved == "" {
		return emit(ui.StatusMsg{Text: fmt.Sprintf("unresolved link: %s", l.display)})
	}
	return emit(ui.OpenNoteMsg{Path: l.resolved})
}

func (m *Model) View() string {
	body := m.vp.View()
	if m.height <= 0 {
		return body
	}
	return body + "\n" + m.footer()
}

// footer draws the single HUD row: note path on the left, the current
// link indicator in the middle, and scroll position on the right.
func (m *Model) footer() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Theme.Dim))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Theme.Accent))

	left := dim.Render(m.path)
	right := dim.Render(fmt.Sprintf("%3d%%", int(m.vp.ScrollPercent()*100)))

	mid := ""
	if len(m.links) > 0 && m.current >= 0 {
		l := m.links[m.current]
		target := l.resolved
		if target == "" {
			target = l.display + " (?)"
		}
		mid = accent.Render(fmt.Sprintf("→ %s (%d/%d)", target, m.current+1, len(m.links)))
	} else {
		mid = dim.Render("no links")
	}

	sep := "  "
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(sep) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 0 {
		// Not enough room for everything; drop the middle section.
		mid = ""
		gap = m.width - lipgloss.Width(left) - lipgloss.Width(right)
	}
	if gap < 0 {
		gap = 0
	}
	line := left + sep + mid + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
}

func emit(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// stripFrontmatter removes a leading YAML frontmatter block so it is
// not rendered as document text. Metadata from it (title, tags) is
// already surfaced through the vault index.
func stripFrontmatter(src string) string {
	rest, ok := strings.CutPrefix(src, "---\n")
	if !ok {
		return src
	}
	if end := strings.Index(rest, "\n---\n"); end >= 0 {
		return rest[end+len("\n---\n"):]
	}
	if strings.HasSuffix(rest, "\n---") {
		// Frontmatter closed at EOF: the whole file is metadata.
		return ""
	}
	return src
}
