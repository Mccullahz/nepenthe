package graphview

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/keymap"
	"github.com/mccullahz/nepenthe-cli/internal/ui"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func runeWidth(r rune) int {
	// Mirror the renderer's width rules for the glyphs graphview emits.
	if r >= 0x1100 && (r <= 0x115f ||
		(r >= 0x2e80 && r <= 0xa4cf) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0x1f300 && r <= 0x1faff)) {
		return 2
	}
	return 1
}

func visibleWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func sampleGraph() *vault.Graph {
	return &vault.Graph{
		Nodes: []vault.Node{
			{ID: 0, Path: "a.md", Title: "Alpha", Degree: 2, Base: ""},
			{ID: 1, Path: "b.md", Title: "Beta", Degree: 2, Base: "b"},
			{ID: 2, Path: "c.md", Title: "Gamma", Degree: 1, Base: "c"},
		},
		Edges: []vault.Edge{{From: 0, To: 1}, {From: 1, To: 2}, {From: 2, To: 0}},
	}
}

func newModel(w, h int) *Model {
	m := New(config.Default(), sampleGraph())
	m.SetSize(w, h)
	return m
}

func TestViewDimensionsMatchSize(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}, {120, 40}, {20, 5}} {
		m := newModel(size[0], size[1])
		// Let the layout run a little so nodes have real positions.
		for i := 0; i < 30; i++ {
			m.Update(ui.GraphTickMsg{})
		}
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != size[1] {
			t.Errorf("size %v: got %d lines, want %d", size, len(lines), size[1])
		}
		for i, ln := range lines {
			if w := visibleWidth(stripANSI(ln)); w != size[0] {
				t.Errorf("size %v line %d width = %d, want %d", size, i, w, size[0])
			}
		}
	}
}

func TestEmptyGraphDimensions(t *testing.T) {
	m := New(config.Default(), &vault.Graph{})
	m.SetSize(50, 10)
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 10 {
		t.Fatalf("empty graph: got %d lines, want 10", len(lines))
	}
	for i, ln := range lines {
		if w := visibleWidth(stripANSI(ln)); w != 50 {
			t.Errorf("empty graph line %d width = %d, want 50", i, w)
		}
	}
}

func TestZeroSize(t *testing.T) {
	m := New(config.Default(), sampleGraph())
	m.SetSize(0, 0)
	if got := m.View(); got != "" {
		t.Errorf("zero size View = %q, want empty", got)
	}
}

func TestOpenNodeEmitsMsg(t *testing.T) {
	m := newModel(80, 24)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("OpenNode produced no command")
	}
	msg := cmd()
	open, ok := msg.(ui.OpenNoteMsg)
	if !ok {
		t.Fatalf("got %T, want OpenNoteMsg", msg)
	}
	if open.Path == "" {
		t.Error("OpenNoteMsg has empty path")
	}
}

func TestGraphDoesNotQuitOnEscOrQ(t *testing.T) {
	// Quitting is :q-only now; bare esc/q on the graph must be inert (no
	// BackMsg/quit), so the app can't be left by a stray keypress.
	m := newModel(80, 24)
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			if _, ok := cmd().(ui.BackMsg); ok {
				t.Fatalf("key %v emitted BackMsg; graph should only exit via :q", k)
			}
		}
	}
}

func TestBackRemainsRebindable(t *testing.T) {
	// The Back action has no default key, but binding one restores a
	// single-key close for anyone who wants it.
	m := newModel(80, 24)
	m.cfg.Keymap.Set(keymap.Back, "q")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("rebound Back produced no command")
	}
	if _, ok := cmd().(ui.BackMsg); !ok {
		t.Fatalf("got %T, want BackMsg", cmd())
	}
}

func TestSwitchBaseNoBases(t *testing.T) {
	m := newModel(80, 24)
	cmd := m.switchBase()
	msg := cmd()
	st, ok := msg.(ui.StatusMsg)
	if !ok || st.Text != "no bases" {
		t.Fatalf("got %v, want StatusMsg{no bases}", msg)
	}
}

func TestSwitchBaseCycles(t *testing.T) {
	m := newModel(80, 24)
	m.SetBases([]string{"", "b", "c"})
	m.SetActiveBase("")
	msg := m.switchBase()()
	sb, ok := msg.(ui.SwitchBaseMsg)
	if !ok || sb.Base != "b" {
		t.Fatalf("first cycle got %v, want base b", msg)
	}
	m.SetActiveBase("c")
	msg = m.switchBase()()
	sb, _ = msg.(ui.SwitchBaseMsg)
	if sb.Base != "" {
		t.Fatalf("wrap cycle got %q, want \"\"", sb.Base)
	}
}

func TestSelectionReachesEveryNode(t *testing.T) {
	m := newModel(80, 24)
	for i := 0; i < 30; i++ {
		m.Update(ui.GraphTickMsg{})
	}
	seen := map[int]bool{}
	km := m.cfg.Keymap
	nextKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(km[keymap.NextNode][0])}
	for i := 0; i < len(m.g.Nodes)*2; i++ {
		seen[m.sel] = true
		m.Update(nextKey)
	}
	if len(seen) != len(m.g.Nodes) {
		t.Errorf("selection cycle reached %d/%d nodes", len(seen), len(m.g.Nodes))
	}
}

func TestSetGraphPreservesSelectionBounds(t *testing.T) {
	m := newModel(80, 24)
	m.sel = 2
	m.SetGraph(&vault.Graph{
		Nodes: []vault.Node{{ID: 0, Path: "a.md", Title: "A"}},
	})
	if m.sel != 0 {
		t.Errorf("sel = %d after shrink, want 0", m.sel)
	}
}

func TestFuzzyScore(t *testing.T) {
	// Empty query matches anything.
	if _, ok := fuzzyScore("", "Anything"); !ok {
		t.Errorf("empty query should match")
	}
	// Contiguous substring matches and outranks a scattered subsequence.
	subScore, ok := fuzzyScore("lph", "Alpha")
	if !ok {
		t.Fatalf("substring should match")
	}
	seqScore, ok := fuzzyScore("apa", "Alpha") // a-l-p-h-a subsequence
	if !ok {
		t.Fatalf("subsequence should match")
	}
	if subScore <= seqScore {
		t.Errorf("substring score %d should beat subsequence score %d", subScore, seqScore)
	}
	// Non-match.
	if _, ok := fuzzyScore("zzz", "Alpha"); ok {
		t.Errorf("zzz should not match Alpha")
	}
	// Case-insensitive.
	if _, ok := fuzzyScore("ALPHA", "alpha"); !ok {
		t.Errorf("match should be case-insensitive")
	}
}

func TestSearchRanksAndFlies(t *testing.T) {
	m := newModel(80, 24)
	for i := 0; i < 30; i++ {
		m.Update(ui.GraphTickMsg{})
	}
	// Open search and type "gam" -> should match Gamma (node 2).
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.searching {
		t.Fatalf("expected search mode after /")
	}
	for _, r := range "gam" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.matches) == 0 || m.searchPool[m.matches[0]].Title != "Gamma" {
		t.Fatalf("top match = %v, want Gamma", m.matches)
	}
	// Enter emits a RevealNoteMsg for the note (the shell routes it back so
	// the base can widen first); search closes.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searching {
		t.Errorf("search should close on enter")
	}
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	reveal, ok := cmd().(ui.RevealNoteMsg)
	if !ok || reveal.Path != "c.md" {
		t.Fatalf("got %#v, want RevealNoteMsg{Path: c.md}", cmd())
	}
	// Delivering it back flies to Gamma.
	m.Update(reveal)
	if m.g.Nodes[m.sel].Title != "Gamma" {
		t.Errorf("selection = %q, want Gamma", m.g.Nodes[m.sel].Title)
	}
}

func TestSearchFolderScopesBase(t *testing.T) {
	m := newModel(80, 24)
	// Supply a whole-vault index with a folder entry.
	m.SetSearchIndex([]ui.SearchEntry{
		{Path: "projects", Title: "projects", IsFolder: true},
		{Path: "projects/web/api.md", Title: "API", Base: "projects"},
	})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "projects" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// The folder should rank first; enter scopes to it via SwitchBaseMsg.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	sb, ok := cmd().(ui.SwitchBaseMsg)
	if !ok || sb.Base != "projects" {
		t.Fatalf("got %#v, want SwitchBaseMsg{Base: projects}", cmd())
	}
}

func TestSearchSpansSuppliedIndex(t *testing.T) {
	// A note absent from the current graph but present in the index is
	// still findable (search spans all bases, not just the view).
	m := newModel(80, 24)
	m.SetSearchIndex([]ui.SearchEntry{
		{Path: "daily/mon.md", Title: "Monday Standup", Base: "daily"},
	})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	for _, r := range "standup" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.matches) != 1 || m.searchPool[m.matches[0]].Path != "daily/mon.md" {
		t.Fatalf("expected to find daily/mon.md by title across bases, got %v", m.matches)
	}
}

func TestSearchOverlayKeepsDimensions(t *testing.T) {
	m := newModel(60, 16)
	for i := 0; i < 20; i++ {
		m.Update(ui.GraphTickMsg{})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 16 {
		t.Fatalf("overlay: got %d lines, want 16", len(lines))
	}
	for i, ln := range lines {
		if w := visibleWidth(stripANSI(ln)); w != 60 {
			t.Errorf("overlay line %d width = %d, want 60", i, w)
		}
	}
}

func TestToggleFocus(t *testing.T) {
	m := newModel(80, 24)
	start := m.focus
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if m.focus == start {
		t.Errorf("focus should toggle on 'f'")
	}
}

func TestSearchTabCyclesMatches(t *testing.T) {
	m := newModel(80, 24) // sampleGraph: Alpha, Beta, Gamma
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	// Empty query -> all three entries match; selection starts at 0.
	if len(m.matches) < 3 {
		t.Fatalf("expected all entries to match empty query, got %d", len(m.matches))
	}
	if m.matchSel != 0 {
		t.Fatalf("initial matchSel = %d, want 0", m.matchSel)
	}
	// Tab advances the highlight; wraps around.
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.matchSel != 1 {
		t.Errorf("after Tab matchSel = %d, want 1", m.matchSel)
	}
	// Shift+Tab goes back.
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.matchSel != 0 {
		t.Errorf("after Shift+Tab matchSel = %d, want 0", m.matchSel)
	}
	// Wrap backward from 0 to last.
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.matchSel != len(m.matches)-1 {
		t.Errorf("wrap-back matchSel = %d, want %d", m.matchSel, len(m.matches)-1)
	}
	// Enter selects the highlighted entry (still in search until then).
	if !m.searching {
		t.Fatal("should still be searching before enter")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searching {
		t.Error("enter should close search")
	}
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
}
