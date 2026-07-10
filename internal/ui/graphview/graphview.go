// Package graphview renders the vault's link graph as a navigable 3D space
// and is the app's home screen. It owns camera and selection state and drives
// a force-directed layout to a resting point, delegating all math and drawing
// to internal/graph3d.
package graphview

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/graph3d"
	"github.com/mccullahz/nepenthe-cli/internal/keymap"
	"github.com/mccullahz/nepenthe-cli/internal/ui"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

// frame is the animation tick interval (~30fps).
const frameInterval = time.Second / 30

// energyThreshold is the kinetic energy below which the layout is considered
// settled and animation may stop.
const energyThreshold = 0.02

// tickMsg drives the layout/camera animation loop.
type tickMsg struct{}

// Model is the 3D graph view.
type Model struct {
	cfg *config.Config
	g   *vault.Graph

	layout *graph3d.Layout
	cam    *graph3d.Camera

	sel        int
	showLabels bool
	focus      bool

	// Fly-to search overlay: when searching, keystrokes filter the search
	// index and enter flies to the chosen note (or scopes to a folder).
	searching  bool
	search     textinput.Model
	index      []ui.SearchEntry // whole-vault index supplied by the shell
	indexSet   bool
	searchPool []ui.SearchEntry // entries being searched this session
	matches    []int            // indices into searchPool, best first
	matchSel   int              // highlighted row in matches

	w, h int

	// ticking guards against tick storms: at most one tick is ever
	// outstanding. It is set when a tick is scheduled and cleared when one
	// arrives.
	ticking bool
	// camMoving tracks whether the camera is still easing toward its target.
	camMoving bool

	// bases is the set of known base names ("" = whole vault); basesSet
	// records whether the shell ever supplied them.
	bases      []string
	basesSet   bool
	activeBase string
}

// New builds the graph view for an already-resolved graph.
func New(cfg *config.Config, g *vault.Graph) *Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "note or folder…"
	m := &Model{
		cfg:        cfg,
		g:          g,
		showLabels: cfg.Graph.ShowLabels,
		focus:      cfg.Graph.Focus,
		search:     ti,
	}
	m.rebuild(nil)
	m.resetView()
	return m
}

// rebuild constructs a fresh layout and camera from the current graph,
// optionally reusing previous positions (matched by node path) so a rescan
// does not scatter surviving nodes.
func (m *Model) rebuild(prev map[string]graph3d.Vec3) {
	paths := make([]string, len(m.g.Nodes))
	for i, n := range m.g.Nodes {
		paths[i] = n.Path
	}
	edges := make([][2]int, 0, len(m.g.Edges))
	for _, e := range m.g.Edges {
		edges = append(edges, [2]int{e.From, e.To})
	}
	params := graph3d.Params{
		LinkDistance: m.cfg.Graph.LinkDistance,
		Repulsion:    m.cfg.Graph.Repulsion,
	}
	m.layout = graph3d.NewLayout(paths, edges, params, prev)
	if m.cam == nil {
		m.cam = graph3d.NewCamera(m.cfg.Graph.FOV)
	}
	m.clampSel()
}

func (m *Model) clampSel() {
	if m.sel >= len(m.g.Nodes) {
		m.sel = len(m.g.Nodes) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

// resetView frames the whole graph: centers on the centroid and sets the
// distance so the bounding sphere fits the viewport.
func (m *Model) resetView() {
	if m.cam == nil || m.layout == nil {
		return
	}
	m.cam.TargetYaw = 0.6
	m.cam.TargetPitch = 0.35
	m.cam.Frame(m.layout.Centroid(), m.layout.BoundingRadius(), float64(m.w), float64(m.h))
}

// SetGraph replaces the graph data (after rescans or base switches),
// preserving the positions of nodes that still exist so the layout does not
// explode. It cannot return a command (frozen signature); animation resumes
// on the next tick or key press.
func (m *Model) SetGraph(g *vault.Graph) {
	var prev map[string]graph3d.Vec3
	if m.layout != nil {
		prev = m.layout.PositionMap()
	}
	m.g = g
	m.rebuild(prev)
}

// SetBases records the known base names ("" meaning the whole vault). The
// shell wires this; SwitchBase cycles through the list.
func (m *Model) SetBases(names []string) {
	m.bases = append(m.bases[:0], names...)
	m.basesSet = true
}

// SetActiveBase records which base is currently shown so SwitchBase advances
// from the right place.
func (m *Model) SetActiveBase(name string) { m.activeBase = name }

// SetSearchIndex supplies the whole-vault "go to" index (all notes and
// folders) so fuzzy search spans every base, not just the one in view.
func (m *Model) SetSearchIndex(entries []ui.SearchEntry) {
	m.index = entries
	m.indexSet = true
}

// searchEntries is what a fresh search ranks over: the shell-supplied
// whole-vault index if present, else the notes in the current graph (so the
// view still searches usefully on its own).
func (m *Model) searchEntries() []ui.SearchEntry {
	if m.indexSet {
		return m.index
	}
	entries := make([]ui.SearchEntry, len(m.g.Nodes))
	for i, n := range m.g.Nodes {
		entries[i] = ui.SearchEntry{Path: n.Path, Title: n.Title, Base: n.Base}
	}
	return entries
}

func (m *Model) Init() tea.Cmd { return m.tick() }

func (m *Model) SetSize(w, h int) {
	m.w, m.h = w, h
	if sw := w - 12; sw > 4 {
		m.search.Width = sw
	}
	m.resetView()
}

// tick schedules one animation frame, but only if none is already pending
// and there is work to do (layout still moving or camera still easing).
func (m *Model) tick() tea.Cmd {
	if m.ticking || !m.active() {
		return nil
	}
	m.ticking = true
	return tea.Tick(frameInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// active reports whether animation should continue.
func (m *Model) active() bool {
	if m.layout != nil && m.layout.Energy() > energyThreshold {
		return true
	}
	return m.camMoving
}

func (m *Model) Update(msg tea.Msg) (ui.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.ticking = false
		if m.layout != nil {
			// A couple of iterations per frame keeps large graphs cheap
			// while still converging at a visible pace.
			m.layout.Step(2)
		}
		if m.cam != nil {
			m.camMoving = m.cam.EaseStep()
		}
		return m, m.tick()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case ui.RevealNoteMsg:
		// The shell has already widened the base if needed; fly to the note.
		return m.flyToPath(msg.Path)
	}
	return m, nil
}

// flyToPath selects and frames the node with the given path, if it is in the
// current graph, restarting the animation loop.
func (m *Model) flyToPath(path string) (ui.View, tea.Cmd) {
	for i, n := range m.g.Nodes {
		if n.Path == path {
			m.flyTo(i)
			return m, m.tick()
		}
	}
	return m, nil
}

func (m *Model) handleKey(key tea.KeyMsg) (ui.View, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(key)
	}
	km := m.cfg.Keymap
	const orbitStep = 0.20
	const pitchStep = 0.15
	switch {
	case km.Matches(key, keymap.Search):
		return m, m.openSearch()
	case km.Matches(key, keymap.ToggleFocus):
		m.focus = !m.focus
		return m, nil
	case km.Matches(key, keymap.OrbitLeft):
		m.cam.OrbitBy(-orbitStep, 0)
	case km.Matches(key, keymap.OrbitRight):
		m.cam.OrbitBy(orbitStep, 0)
	case km.Matches(key, keymap.OrbitUp):
		m.cam.OrbitBy(0, pitchStep)
	case km.Matches(key, keymap.OrbitDown):
		m.cam.OrbitBy(0, -pitchStep)
	case km.Matches(key, keymap.ZoomIn):
		m.cam.ZoomBy(0.85)
	case km.Matches(key, keymap.ZoomOut):
		m.cam.ZoomBy(1.0 / 0.85)
	case km.Matches(key, keymap.NextNode):
		m.cycleSelection(1)
	case km.Matches(key, keymap.PrevNode):
		m.cycleSelection(-1)
	case km.Matches(key, keymap.OpenNode):
		if len(m.g.Nodes) > 0 {
			path := m.g.Nodes[m.sel].Path
			return m, func() tea.Msg { return ui.OpenNoteMsg{Path: path} }
		}
	case km.Matches(key, keymap.ResetView):
		m.resetView()
	case km.Matches(key, keymap.ToggleLabels):
		m.showLabels = !m.showLabels
	case km.Matches(key, keymap.SwitchBase):
		return m, m.switchBase()
	case km.Matches(key, keymap.Back):
		return m, func() tea.Msg { return ui.BackMsg{} }
	default:
		return m, nil
	}
	// Camera or selection changed; make sure the animation loop is running.
	m.camMoving = true
	return m, m.tick()
}

// switchBase advances to the next known base and asks the shell to switch. If
// bases were never supplied it reports a status message instead.
func (m *Model) switchBase() tea.Cmd {
	if !m.basesSet || len(m.bases) == 0 {
		return func() tea.Msg { return ui.StatusMsg{Text: "no bases"} }
	}
	idx := 0
	for i, b := range m.bases {
		if b == m.activeBase {
			idx = i
			break
		}
	}
	next := m.bases[(idx+1)%len(m.bases)]
	return func() tea.Msg { return ui.SwitchBaseMsg{Base: next} }
}

// cycleSelection moves the selection by dir (+1/-1) through nodes ordered by
// their current projected distance from screen center. The ordering is a full
// permutation recomputed at press time, so every node is reachable and the
// result is deterministic.
func (m *Model) cycleSelection(dir int) {
	n := len(m.g.Nodes)
	if n == 0 {
		return
	}
	order := m.selectionOrder()
	pos := 0
	for i, id := range order {
		if id == m.sel {
			pos = i
			break
		}
	}
	pos = ((pos+dir)%n + n) % n
	m.sel = order[pos]
	// Auto-center: pan the camera so the newly selected node sits at the
	// middle of the screen, keeping it readable as you cycle.
	m.cam.LookAt(m.layout.Positions()[m.sel])
}

// openSearch enters the fly-to search overlay, snapshotting the pool of
// entries (whole vault) to rank against.
func (m *Model) openSearch() tea.Cmd {
	m.searching = true
	m.search.SetValue("")
	m.matchSel = 0
	m.searchPool = m.searchEntries()
	m.updateMatches()
	return m.search.Focus()
}

func (m *Model) closeSearch() {
	m.searching = false
	m.search.Blur()
}

// handleSearchKey handles keys while the search overlay is open.
func (m *Model) handleSearchKey(key tea.KeyMsg) (ui.View, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.closeSearch()
		return m, nil
	case "enter":
		if len(m.matches) > 0 {
			e := m.searchPool[m.matches[m.matchSel]]
			m.closeSearch()
			if e.IsFolder {
				// Scope the graph to the chosen folder (any depth).
				return m, func() tea.Msg { return ui.SwitchBaseMsg{Base: e.Path} }
			}
			// Let the shell widen the base if needed, then fly to the note.
			return m, func() tea.Msg { return ui.RevealNoteMsg{Path: e.Path} }
		}
		m.closeSearch()
		return m, nil
	case "down", "ctrl+n":
		if n := len(m.matches); n > 0 {
			m.matchSel = (m.matchSel + 1) % n
		}
		return m, nil
	case "up", "ctrl+p":
		if n := len(m.matches); n > 0 {
			m.matchSel = (m.matchSel - 1 + n) % n
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(key)
	m.updateMatches()
	return m, cmd
}

// updateMatches recomputes the ranked match list for the current query over
// the whole-vault search pool.
func (m *Model) updateMatches() {
	q := m.search.Value()
	type scored struct {
		id, score int
	}
	var hits []scored
	for i, e := range m.searchPool {
		if s, ok := entryScore(q, e); ok {
			hits = append(hits, scored{i, s})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		return m.searchPool[hits[a].id].Title < m.searchPool[hits[b].id].Title
	})
	m.matches = m.matches[:0]
	for _, h := range hits {
		m.matches = append(m.matches, h.id)
	}
	if m.matchSel >= len(m.matches) {
		m.matchSel = 0
	}
}

// entryScore ranks a search entry against the query. Notes match on title or
// (at a small penalty) their path, so "web" surfaces both a projects/web
// folder and the notes inside it; folders match on their path.
func entryScore(q string, e ui.SearchEntry) (int, bool) {
	if e.IsFolder {
		return fuzzyScore(q, e.Path)
	}
	ts, tok := fuzzyScore(q, e.Title)
	ps, pok := fuzzyScore(q, e.Path)
	switch {
	case tok && pok:
		if ts >= ps-200 {
			return ts, true
		}
		return ps - 200, true
	case tok:
		return ts, true
	case pok:
		return ps - 200, true
	default:
		return 0, false
	}
}

// flyTo selects a node and frames the camera on it (fly-and-zoom).
func (m *Model) flyTo(id int) {
	if id < 0 || id >= len(m.g.Nodes) || m.cam == nil || m.layout == nil {
		return
	}
	m.sel = id
	radius := m.cfg.Graph.LinkDistance * 4
	if radius < 2 {
		radius = 2
	}
	m.cam.Frame(m.layout.Positions()[id], radius, float64(m.w), float64(m.h))
	m.camMoving = true
}

// fuzzyScore reports whether query q fuzzy-matches title t, and a score
// (higher is better). An empty query matches everything. A contiguous
// substring outranks a scattered subsequence, and earlier / more
// contiguous matches score higher.
func fuzzyScore(q, t string) (int, bool) {
	if q == "" {
		return 0, true
	}
	ql, tl := strings.ToLower(q), strings.ToLower(t)
	if idx := strings.Index(tl, ql); idx >= 0 {
		return 2000 - idx, true // contiguous substring: best
	}
	ti, score, streak := 0, 0, 0
	for i := 0; i < len(ql); i++ {
		found := false
		for ti < len(tl) {
			c := tl[ti]
			ti++
			if c == ql[i] {
				streak++
				score += streak
				found = true
				break
			}
			streak = 0
		}
		if !found {
			return 0, false
		}
	}
	return score, true
}

// selectionOrder returns node indices sorted by projected distance from the
// screen center (nearest first), with node ID as a deterministic tie-break.
func (m *Model) selectionOrder() []int {
	n := len(m.g.Nodes)
	order := make([]int, n)
	dist := make([]float64, n)
	positions := m.layout.Positions()
	cx, cy := float64(m.w)/2, float64(m.h)/2
	for i := 0; i < n; i++ {
		order[i] = i
		x, y, _, ok := m.cam.Project(positions[i], m.w, m.h)
		if !ok {
			dist[i] = math.Inf(1)
			continue
		}
		dist[i] = math.Hypot(float64(x)-cx, float64(y)-cy)
	}
	sort.SliceStable(order, func(a, b int) bool {
		ia, ib := order[a], order[b]
		if dist[ia] != dist[ib] {
			return dist[ia] < dist[ib]
		}
		return ia < ib
	})
	return order
}

func (m *Model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return ""
	}
	if len(m.g.Nodes) == 0 {
		return m.emptyState()
	}

	titles := make([]string, len(m.g.Nodes))
	degrees := make([]int, len(m.g.Nodes))
	bases := make([]string, len(m.g.Nodes))
	for i, n := range m.g.Nodes {
		titles[i] = n.Title
		degrees[i] = n.Degree
		bases[i] = n.Base
	}
	edges := make([][2]int, 0, len(m.g.Edges))
	for _, e := range m.g.Edges {
		edges = append(edges, [2]int{e.From, e.To})
	}
	scene := graph3d.Scene{
		Cam:        m.cam,
		Pos:        m.layout.Positions(),
		Titles:     titles,
		Degrees:    degrees,
		Bases:      bases,
		Edges:      edges,
		Selected:   m.sel,
		ShowLabels: m.showLabels,
		Focus:      m.focus,
		Accent:     m.cfg.Theme.Accent,
		Dim:        m.cfg.Theme.Dim,
	}
	out := graph3d.Render(scene, m.w, m.h).String()
	if m.searching {
		out = m.overlaySearch(out)
	}
	return out
}

// overlaySearch splices the fly-to search panel over the top rows of the
// rendered graph. It replaces whole lines (each padded to the exact width)
// so the frame keeps its dimensions.
func (m *Model) overlaySearch(base string) string {
	lines := strings.Split(base, "\n")
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Theme.Accent)).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.Theme.Dim))
	sel := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("231")).
		Background(lipgloss.Color(m.cfg.Theme.Accent))

	panel := []string{m.padLine(accent.Render("  /") + m.search.View())}
	if len(m.matches) == 0 {
		panel = append(panel, m.padLine(dim.Render("    no matching notes or folders")))
	} else {
		const limit = 7
		start := 0
		if m.matchSel >= limit {
			start = m.matchSel - limit + 1
		}
		for i := start; i < len(m.matches) && i < start+limit; i++ {
			e := m.searchPool[m.matches[i]]
			var text string
			if e.IsFolder {
				text = e.Path + "/   base" // scoping target
			} else {
				text = e.Title
				if e.Base != "" {
					text += "   " + e.Base
				}
			}
			if i == m.matchSel {
				panel = append(panel, m.padLine(sel.Render("  › "+text)))
			} else {
				panel = append(panel, m.padLine(dim.Render("    "+text)))
			}
		}
	}
	for i := 0; i < len(panel) && i < len(lines); i++ {
		lines[i] = panel[i]
	}
	return strings.Join(lines, "\n")
}

// padLine makes a styled line exactly m.w visible columns wide.
func (m *Model) padLine(s string) string {
	w := lipgloss.Width(s)
	switch {
	case w < m.w:
		return s + strings.Repeat(" ", m.w-w)
	case w > m.w:
		return lipgloss.NewStyle().MaxWidth(m.w).Render(s)
	default:
		return s
	}
}

// emptyState renders a centered message filling the exact viewport when the
// graph has no nodes.
func (m *Model) emptyState() string {
	msg := "empty vault — :new <name> to create your first note"
	return graph3d.EmptyState(m.w, m.h, msg, m.cfg.Theme.Dim).String()
}
