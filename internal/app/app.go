// Package app is the root Bubble Tea model: it owns the vault and the
// view stack, routes messages between views, runs the ':' command line,
// and draws the status bar. Views never import this package — they
// communicate by emitting messages from internal/ui.
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/keymap"
	"github.com/mccullahz/nepenthe-cli/internal/ui"
	"github.com/mccullahz/nepenthe-cli/internal/ui/graphview"
	"github.com/mccullahz/nepenthe-cli/internal/ui/noteview"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

type editorFinishedMsg struct {
	path string
	err  error
}

type clearStatusMsg struct{ seq int }

// App is the root model.
type App struct {
	cfg   *config.Config
	vault *vault.Vault

	// stack[0] is always the graph view; notes and editors push on top.
	stack  []ui.View
	width  int
	height int
	base   string // active knowledge base, "" = whole vault

	cmdMode bool
	cmd     textinput.Model

	status    string
	statusSeq int

	showHelp bool
}

// New builds the app. initialStatus, if non-empty, is shown in the
// status bar on startup (e.g. a config load warning).
func New(cfg *config.Config, v *vault.Vault, initialStatus string) *App {
	ti := textinput.New()
	ti.Prompt = ":"
	gv := graphview.New(cfg, v.Graph(""))
	gv.SetBases(baseNames(v))
	gv.SetActiveBase("")
	return &App{
		cfg:    cfg,
		vault:  v,
		stack:  []ui.View{gv},
		cmd:    ti,
		status: initialStatus,
	}
}

func baseNames(v *vault.Vault) []string {
	bases := v.Bases()
	names := make([]string, len(bases))
	for i, b := range bases {
		names[i] = b.Name
	}
	return names
}

func (a *App) Init() tea.Cmd { return a.stack[0].Init() }

func (a *App) top() ui.View { return a.stack[len(a.stack)-1] }

func (a *App) contentHeight() int {
	h := a.height - 1
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) refreshGraph() {
	if gv, ok := a.stack[0].(*graphview.Model); ok {
		gv.SetGraph(a.vault.Graph(a.base))
		gv.SetBases(baseNames(a.vault))
		gv.SetActiveBase(a.base)
	}
}

// reloadTopNote swaps a stale note viewer for a fresh one (used after
// saves, external edits, and pops back from the editor).
func (a *App) reloadTopNote() tea.Cmd {
	nv, ok := a.top().(*noteview.Model)
	if !ok {
		return nil
	}
	fresh := noteview.New(a.cfg, a.vault, nv.Path())
	fresh.SetSize(a.width, a.contentHeight())
	a.stack[len(a.stack)-1] = fresh
	return fresh.Init()
}

func (a *App) setStatus(s string) tea.Cmd {
	a.status = s
	a.statusSeq++
	seq := a.statusSeq
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return clearStatusMsg{seq: seq}
	})
}

func emit(m tea.Msg) tea.Cmd {
	return func() tea.Msg { return m }
}

func (a *App) Update(m tea.Msg) (tea.Model, tea.Cmd) {
	switch m := m.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.cmd.Width = a.width - 2
		for _, v := range a.stack {
			v.SetSize(a.width, a.contentHeight())
		}
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(m)

	case ui.OpenNoteMsg:
		nv := noteview.New(a.cfg, a.vault, m.Path)
		nv.SetSize(a.width, a.contentHeight())
		a.stack = append(a.stack, nv)
		return a, nv.Init()

	case ui.EditExternalMsg:
		full := filepath.Join(a.vault.Root, filepath.FromSlash(m.Path))
		parts := strings.Fields(a.cfg.EditorCommand())
		c := exec.Command(parts[0], append(parts[1:], full)...)
		path := m.Path
		return a, tea.ExecProcess(c, func(err error) tea.Msg {
			return editorFinishedMsg{path: path, err: err}
		})

	case editorFinishedMsg:
		if m.err != nil {
			return a, a.setStatus("editor: " + m.err.Error())
		}
		a.vault.Rescan()
		a.refreshGraph()
		return a, tea.Batch(a.reloadTopNote(), a.setStatus("reloaded "+m.path))

	case ui.BackMsg:
		if len(a.stack) == 1 {
			return a, tea.Quit
		}
		a.stack = a.stack[:len(a.stack)-1]
		return a, a.reloadTopNote()

	case ui.NavBackMsg:
		// esc in read mode: step back one note in the link trail, but never
		// fall through to the graph — that is what :q is for.
		if len(a.stack) >= 3 {
			a.stack = a.stack[:len(a.stack)-1]
			return a, a.reloadTopNote()
		}
		return a, a.setStatus(":q to close this note")

	case ui.StatusMsg:
		return a, a.setStatus(m.Text)

	case ui.VaultChangedMsg:
		a.vault.Rescan()
		a.refreshGraph()
		return a, nil

	case ui.SwitchBaseMsg:
		a.base = m.Base
		a.refreshGraph()
		name := m.Base
		if name == "" {
			name = "all notes"
		}
		return a, a.setStatus("base: " + name)

	case clearStatusMsg:
		if m.seq == a.statusSeq {
			a.status = ""
		}
		return a, nil
	}
	return a.routeToTop(m)
}

func (a *App) routeToTop(m tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := a.top().Update(m)
	a.stack[len(a.stack)-1] = next
	return a, cmd
}

func (a *App) handleKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.String() == "ctrl+c" {
		return a, tea.Quit
	}
	if a.cmdMode {
		switch m.String() {
		case "esc":
			a.cmdMode = false
			a.cmd.Blur()
			return a, nil
		case "enter":
			line := strings.TrimSpace(a.cmd.Value())
			a.cmdMode = false
			a.cmd.Blur()
			a.cmd.SetValue("")
			return a, a.runCommand(line)
		}
		var cmd tea.Cmd
		a.cmd, cmd = a.cmd.Update(m)
		return a, cmd
	}
	if a.showHelp {
		a.showHelp = false
		return a, nil
	}
	km := a.cfg.Keymap
	switch {
	case km.Matches(m, keymap.Quit):
		return a, tea.Quit
	case km.Matches(m, keymap.Help):
		a.showHelp = true
		return a, nil
	case km.Matches(m, keymap.Command):
		a.cmdMode = true
		a.cmd.SetValue("")
		return a, a.cmd.Focus()
	}
	return a.routeToTop(m)
}

func (a *App) runCommand(line string) tea.Cmd {
	if line == "" {
		return nil
	}
	fields := strings.Fields(line)
	switch fields[0] {
	case "q", "quit":
		// Vim-style: :q closes the current note (back to whatever is
		// beneath it); on the graph it quits the app.
		if _, ok := a.top().(*noteview.Model); ok {
			return emit(ui.BackMsg{})
		}
		return tea.Quit

	case "new":
		if len(fields) < 2 {
			return a.setStatus("usage: :new <path>")
		}
		path := fields[1]
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		if err := a.vault.Create(path); err != nil {
			return a.setStatus(err.Error())
		}
		a.refreshGraph()
		return emit(ui.OpenNoteMsg{Path: path})

	case "open":
		if len(fields) < 2 {
			return a.setStatus("usage: :open <path>")
		}
		return emit(ui.OpenNoteMsg{Path: fields[1]})

	case "delete":
		if len(fields) < 2 {
			return a.setStatus("usage: :delete <path>")
		}
		if err := a.vault.Delete(fields[1]); err != nil {
			return a.setStatus(err.Error())
		}
		a.refreshGraph()
		return a.setStatus("deleted " + fields[1])

	case "base":
		name := ""
		if len(fields) > 1 {
			name = fields[1]
		}
		return emit(ui.SwitchBaseMsg{Base: name})

	case "import":
		if len(fields) < 2 {
			return a.setStatus("usage: :import <src> [dest-dir]")
		}
		dest := ""
		if len(fields) > 2 {
			dest = fields[2]
		}
		if err := a.vault.Import(fields[1], dest); err != nil {
			return a.setStatus(err.Error())
		}
		a.refreshGraph()
		return a.setStatus("imported " + fields[1])

	case "export":
		if len(fields) < 3 {
			return a.setStatus("usage: :export <note|.> <dest>")
		}
		path := fields[1]
		if path == "." {
			path = ""
		}
		if err := a.vault.Export(path, fields[2]); err != nil {
			return a.setStatus(err.Error())
		}
		return a.setStatus("exported to " + fields[2])

	case "set":
		if len(fields) < 3 {
			return a.setStatus("usage: :set <option> <value>   (:set graph.show_labels true, :set theme.accent #ff79c6, ...)")
		}
		return a.setOption(fields[1], strings.Join(fields[2:], " "))

	case "bind":
		if len(fields) < 3 {
			return a.setStatus("usage: :bind <action> <key> [key...]   (:bind zoom_in K +)")
		}
		action := keymap.Action(fields[1])
		valid := false
		for _, known := range keymap.Actions() {
			if known == action {
				valid = true
				break
			}
		}
		if !valid {
			return a.setStatus("unknown action: " + fields[1] + " (see ? for actions)")
		}
		a.cfg.Keymap.Set(action, fields[2:]...)
		return a.setStatus(fmt.Sprintf("bound %s to %s", fields[1], strings.Join(fields[2:], "/")))

	case "rescan":
		if err := a.vault.Rescan(); err != nil {
			return a.setStatus(err.Error())
		}
		a.refreshGraph()
		return a.setStatus(fmt.Sprintf("%d notes indexed", len(a.vault.Notes)))

	default:
		return a.setStatus("unknown command: " + fields[0])
	}
}

// setOption applies a ':set' command to the live config. Config is
// shared by pointer with every view, so changes take effect on the
// next render (glamour theme changes apply to newly opened notes).
func (a *App) setOption(key, val string) tea.Cmd {
	fail := func(err error) tea.Cmd { return a.setStatus("set " + key + ": " + err.Error()) }
	num := func(dst *float64) tea.Cmd {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fail(err)
		}
		*dst = f
		return a.setStatus(key + " = " + val)
	}
	switch key {
	case "editor":
		a.cfg.Editor = val
	case "theme.style":
		a.cfg.Theme.GlamourStyle = val
	case "theme.accent":
		a.cfg.Theme.Accent = val
	case "theme.dim":
		a.cfg.Theme.Dim = val
	case "graph.link_distance":
		return num(&a.cfg.Graph.LinkDistance)
	case "graph.repulsion":
		return num(&a.cfg.Graph.Repulsion)
	case "graph.fov":
		return num(&a.cfg.Graph.FOV)
	case "graph.iterations":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fail(err)
		}
		a.cfg.Graph.Iterations = n
	case "graph.show_labels":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fail(err)
		}
		a.cfg.Graph.ShowLabels = b
	default:
		return a.setStatus("unknown option: " + key)
	}
	return a.setStatus(key + " = " + val)
}

func (a *App) modeName() string {
	if _, ok := a.top().(*noteview.Model); ok {
		return "note"
	}
	return "graph"
}

func (a *App) View() string {
	if a.width == 0 {
		return "loading…"
	}
	content := a.top().View()
	if a.showHelp {
		content = a.renderHelp()
	}
	return content + "\n" + a.bottomBar()
}

func (a *App) bottomBar() string {
	if a.cmdMode {
		return a.cmd.View()
	}
	accent := lipgloss.Color(a.cfg.Theme.Accent)
	dim := lipgloss.Color(a.cfg.Theme.Dim)

	left := lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(accent).
		Padding(0, 1).
		Render("nepenthe " + a.modeName())

	baseName := a.base
	if baseName == "" {
		baseName = "all"
	}
	right := lipgloss.NewStyle().
		Foreground(dim).
		Padding(0, 1).
		Render(fmt.Sprintf("%s • %d notes • ? help", baseName, len(a.vault.Notes)))

	mid := lipgloss.NewStyle().Foreground(dim).Padding(0, 1).Render(a.status)

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + mid + strings.Repeat(" ", gap) + right
}

func (a *App) renderHelp() string {
	km := a.cfg.Keymap
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(a.cfg.Theme.Dim))
	head := lipgloss.NewStyle().Foreground(lipgloss.Color(a.cfg.Theme.Accent)).Bold(true)

	row := func(action keymap.Action, desc string) string {
		return fmt.Sprintf("  %-16s %s", km.Hint(action), desc)
	}
	sections := []string{
		head.Render("Global"),
		row(keymap.Help, "toggle this help"),
		row(keymap.Command, "command line (:new :open :base :import :export :delete :rescan :q)"),
		dim.Render("  :q               quit (from graph) / close note"),
		row(keymap.Quit, "force quit"),
		"",
		head.Render("Graph"),
		row(keymap.OrbitLeft, "orbit left") + dim.Render("   (right/up/down: "+km.Hint(keymap.OrbitRight)+" "+km.Hint(keymap.OrbitUp)+" "+km.Hint(keymap.OrbitDown)+")"),
		row(keymap.ZoomIn, "zoom in") + dim.Render("   (out: "+km.Hint(keymap.ZoomOut)+")"),
		row(keymap.NextNode, "select next node (auto-centers)") + dim.Render("   (prev: "+km.Hint(keymap.PrevNode)+")"),
		row(keymap.Search, "fuzzy-search notes and fly to one"),
		row(keymap.OpenNode, "open selected note"),
		row(keymap.ResetView, "reset camera"),
		row(keymap.ToggleFocus, "toggle focus mode (spotlight neighborhood)"),
		row(keymap.ToggleLabels, "toggle all titles"),
		row(keymap.SwitchBase, "cycle knowledge base"),
		"",
		head.Render("Note (read mode)"),
		row(keymap.FollowLink, "follow selected link") + dim.Render("   (cycle: "+km.Hint(keymap.NextLink)+")"),
		row(keymap.LinkBack, "back through the link trail"),
		row(keymap.Edit, "edit in $EDITOR"),
		dim.Render("  :q               close note (esc never exits read mode)"),
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(a.cfg.Theme.Accent)).
		Padding(1, 3).
		Render(strings.Join(sections, "\n"))
	return lipgloss.Place(a.width, a.contentHeight(), lipgloss.Center, lipgloss.Center, box)
}
