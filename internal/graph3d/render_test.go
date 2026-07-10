package graph3d

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func visibleWidth(s string) int {
	w := 0
	for _, r := range s {
		w += cellWidth(r)
	}
	return w
}

func TestBufferExactDimensions(t *testing.T) {
	b := NewBuffer(20, 6)
	b.put(3, 2, Cell{Ch: '●', FG: "42"})
	out := b.String()
	lines := strings.Split(out, "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	for i, ln := range lines {
		if w := visibleWidth(stripANSI(ln)); w != 20 {
			t.Errorf("line %d width = %d, want 20 (%q)", i, w, stripANSI(ln))
		}
	}
}

func TestRenderDimensions(t *testing.T) {
	paths := []string{"alpha.md", "beta.md", "gamma.md"}
	edges := [][2]int{{0, 1}, {1, 2}}
	l := NewLayout(paths, nil, edges, Params{}, nil)
	l.Step(100)
	cam := NewCamera(70)
	cam.Frame(l.Centroid(), l.BoundingRadius(), 60, 24)
	cam.Dist = cam.TargetDist

	scene := Scene{
		Cam:        cam,
		Pos:        l.Positions(),
		Titles:     []string{"Alpha", "Beta", "Gamma"},
		Degrees:    []int{1, 2, 1},
		Bases:      []string{"", "", ""},
		Edges:      edges,
		Selected:   0,
		ShowLabels: true,
		Accent:     "212",
		Dim:        "240",
	}
	out := Render(scene, 60, 24).String()
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("got %d lines, want 24", len(lines))
	}
	for i, ln := range lines {
		if w := visibleWidth(stripANSI(ln)); w != 60 {
			t.Errorf("line %d width = %d, want 60", i, w)
		}
	}
}

func TestRenderWideRuneLabel(t *testing.T) {
	// A CJK (double-width) title must not break line widths.
	l := NewLayout([]string{"n.md"}, nil, nil, Params{}, nil)
	cam := NewCamera(70)
	cam.Frame(Vec3{}, 1, 40, 12)
	cam.Dist = cam.TargetDist
	scene := Scene{
		Cam:        cam,
		Pos:        l.Positions(),
		Titles:     []string{"日本語ノート"},
		Degrees:    []int{0},
		Bases:      []string{""},
		Edges:      nil,
		Selected:   0,
		ShowLabels: true,
		Accent:     "212",
		Dim:        "240",
	}
	out := Render(scene, 40, 12).String()
	for i, ln := range strings.Split(out, "\n") {
		if w := visibleWidth(stripANSI(ln)); w != 40 {
			t.Errorf("line %d width = %d, want 40", i, w)
		}
	}
}

func TestCellWidth(t *testing.T) {
	cases := []struct {
		r rune
		w int
	}{
		{'a', 1}, {'●', 1}, {'·', 1}, {'◉', 1}, {' ', 1},
		{'日', 2}, {'あ', 2}, {0, 0}, {'\x00', 0},
	}
	for _, c := range cases {
		if got := cellWidth(c.r); got != c.w {
			t.Errorf("cellWidth(%q) = %d, want %d", c.r, got, c.w)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 6); visibleWidth(got) > 6 {
		t.Errorf("truncate too wide: %q (%d)", got, visibleWidth(got))
	}
	if got := truncate("short", 20); got != "short" {
		t.Errorf("truncate = %q, want short", got)
	}
}

func TestEmptySceneNoPanic(t *testing.T) {
	b := Render(Scene{}, 10, 4)
	if len(strings.Split(b.String(), "\n")) != 4 {
		t.Error("empty scene produced wrong height")
	}
	// Zero-size must not panic.
	_ = Render(Scene{}, 0, 0).String()
}

func TestFocusSet(t *testing.T) {
	// 0-1, 1-2, so neighbors of 1 are {0,2}; node 3 is isolated.
	s := Scene{
		Pos:      []Vec3{{}, {}, {}, {}},
		Edges:    [][2]int{{0, 1}, {1, 2}},
		Selected: 1,
	}
	f := focusSet(s)
	want := []bool{true, true, true, false} // 0,1,2 in focus; 3 out
	for i := range want {
		if f[i] != want[i] {
			t.Errorf("focusSet[%d] = %v, want %v", i, f[i], want[i])
		}
	}
}

func TestFocusModeDimsNonNeighbors(t *testing.T) {
	// A star: center 0 linked to 1; node 2 is unrelated and far.
	s := Scene{
		Cam:      NewCamera(70),
		Pos:      []Vec3{{X: 0}, {X: 1}, {X: 8}},
		Titles:   []string{"Center", "Neighbor", "Stranger"},
		Degrees:  []int{1, 1, 0},
		Bases:    []string{"", "", ""},
		Edges:    [][2]int{{0, 1}},
		Selected: 0,
		Focus:    true,
		Accent:   "#7D56F4",
		Dim:      "240",
	}
	s.Cam.Center = Vec3{}
	s.Cam.Dist = 20
	foc := focusSet(s)
	if foc[2] {
		t.Fatalf("stranger should be out of focus")
	}
	// Rendering must still produce exact dimensions with focus on.
	out := Render(s, 40, 12).String()
	lines := strings.Split(out, "\n")
	if len(lines) != 12 {
		t.Fatalf("focus render: got %d lines, want 12", len(lines))
	}
	for i, ln := range lines {
		if w := visibleWidth(stripANSI(ln)); w != 40 {
			t.Errorf("focus render line %d width = %d, want 40", i, w)
		}
	}
}

func TestSlopeGlyph(t *testing.T) {
	cases := []struct {
		x0, y0, x1, y1 int
		want           rune
	}{
		{0, 0, 10, 0, '─'}, // horizontal
		{0, 0, 0, 10, '│'}, // vertical
		{0, 0, 8, 8, '╲'},  // down-right (y grows downward)
		{0, 8, 8, 0, '╱'},  // up-right
		{10, 0, 0, 0, '─'}, // horizontal, reversed
		{8, 8, 0, 0, '╲'},  // down-right, reversed
	}
	for _, c := range cases {
		if got := slopeGlyph(c.x0, c.y0, c.x1, c.y1); got != c.want {
			t.Errorf("slopeGlyph(%d,%d,%d,%d) = %q, want %q", c.x0, c.y0, c.x1, c.y1, got, c.want)
		}
	}
}

func TestAllTitlesLabeledOutsideFocus(t *testing.T) {
	// With focus off, every visible node with a clear cell should be labeled
	// (not just a fixed near-count). Lay out three well-separated nodes.
	s := Scene{
		Cam:        NewCamera(70),
		Pos:        []Vec3{{X: -6, Y: 3}, {X: 0, Y: -3}, {X: 6, Y: 3}},
		Titles:     []string{"AlphaNote", "BetaNote", "GammaNote"},
		Degrees:    []int{0, 0, 0},
		Bases:      []string{"", "", ""},
		Edges:      nil,
		Selected:   0,
		ShowLabels: true,
		Focus:      false,
		Accent:     "#7D56F4",
		Dim:        "240",
	}
	s.Cam.Center = Vec3{}
	s.Cam.Dist = 20
	out := stripANSI(Render(s, 80, 24).String())
	for _, want := range []string{"Alpha", "Beta", "Gamma"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected title %q to be visible, output:\n%s", want, out)
		}
	}
}
