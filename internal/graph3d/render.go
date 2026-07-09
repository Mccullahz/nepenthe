package graph3d

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Cell is one character position in the render buffer. Ch==0 marks the
// second half of a double-width rune to its left and renders as nothing so
// the buffer's visible width always equals its cell count.
type Cell struct {
	Ch   rune
	FG   string // lipgloss color; "" means terminal default
	Bold bool
	Inv  bool // inverse video (used for the selected node)
	prio int8 // draw priority; higher wins when cells contend
	set  bool
}

// Buffer is a fixed-size grid of cells rasterized by the renderer and then
// serialized to a single ANSI string exactly W columns by H rows.
type Buffer struct {
	W, H  int
	cells []Cell
}

// NewBuffer returns a W×H buffer filled with blank cells.
func NewBuffer(w, h int) *Buffer {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	b := &Buffer{W: w, H: h, cells: make([]Cell, w*h)}
	for i := range b.cells {
		b.cells[i].Ch = ' '
	}
	return b
}

func (b *Buffer) in(x, y int) bool { return x >= 0 && x < b.W && y >= 0 && y < b.H }

// put writes c at (x,y) if it is in bounds and outranks whatever is there.
func (b *Buffer) put(x, y int, c Cell) {
	if !b.in(x, y) {
		return
	}
	i := y*b.W + x
	if b.cells[i].set && b.cells[i].prio > c.prio {
		return
	}
	c.set = true
	b.cells[i] = c
}

// At returns a copy of the cell at (x,y), or a blank cell if out of bounds.
func (b *Buffer) At(x, y int) Cell {
	if !b.in(x, y) {
		return Cell{Ch: ' '}
	}
	return b.cells[y*b.W+x]
}

// String serializes the buffer to ANSI, grouping runs of same-styled cells
// so lipgloss is invoked once per run rather than once per cell. Every line
// is exactly W visible columns and there are exactly H lines.
func (b *Buffer) String() string {
	if b.W == 0 || b.H == 0 {
		return ""
	}
	var sb strings.Builder
	for y := 0; y < b.H; y++ {
		var run strings.Builder
		var cur Cell
		haveRun := false
		flush := func() {
			if !haveRun {
				return
			}
			sb.WriteString(styleRun(cur, run.String()))
			run.Reset()
			haveRun = false
		}
		for x := 0; x < b.W; x++ {
			c := b.cells[y*b.W+x]
			if c.Ch == 0 {
				// continuation of a wide rune: contributes no glyph
				continue
			}
			if haveRun && sameStyle(cur, c) {
				run.WriteRune(c.Ch)
				continue
			}
			flush()
			cur = c
			run.WriteRune(c.Ch)
			haveRun = true
		}
		flush()
		if y < b.H-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func sameStyle(a, c Cell) bool {
	return a.FG == c.FG && a.Bold == c.Bold && a.Inv == c.Inv
}

func styleRun(c Cell, s string) string {
	if s == "" {
		return ""
	}
	if c.FG == "" && !c.Bold && !c.Inv {
		return s
	}
	st := lipgloss.NewStyle()
	if c.FG != "" {
		st = st.Foreground(lipgloss.Color(c.FG))
	}
	if c.Bold {
		st = st.Bold(true)
	}
	if c.Inv {
		st = st.Reverse(true)
	}
	return st.Render(s)
}

// EmptyState renders a single centered, truncated message into a blank
// w×h buffer. It guarantees exact dimensions for the graph view's empty and
// too-small cases.
func EmptyState(w, h int, msg, fg string) *Buffer {
	buf := NewBuffer(w, h)
	if w <= 0 || h <= 0 {
		return buf
	}
	text := truncate(msg, w)
	tw := 0
	for _, r := range text {
		tw += cellWidth(r)
	}
	x := (w - tw) / 2
	if x < 0 {
		x = 0
	}
	y := h / 2
	writeRaw(buf, x, y, text, fg, false)
	return buf
}

// Scene is everything the renderer needs to draw one frame. It carries plain
// data (no vault or Bubble Tea types) so the renderer stays unit-testable.
type Scene struct {
	Cam        *Camera
	Pos        []Vec3
	Titles     []string
	Degrees    []int
	Bases      []string
	Edges      [][2]int
	Selected   int
	ShowLabels bool
	// Focus dims every node except the selected one and its direct
	// neighbors and hides edges not touching the selection, cutting the
	// visual "hairball" down to the neighborhood you're looking at.
	Focus  bool
	Accent string
	Dim    string
}

// projected is a node after projection, retained for depth-sorted drawing,
// labeling and selection ordering.
type projected struct {
	id         int
	x, y       int
	depth      float64
	visible    bool
	depthNorm  float64 // 0 = nearest visible node, 1 = farthest
	centerDist float64 // distance from screen center, in cells
}

// Render rasterizes the scene into a w×h buffer: edges first (painter's
// order, far to near), then nodes, then labels and a HUD line. Depth cueing
// (glyph size, brightness and label eligibility all fall off with distance)
// is what conveys the third dimension.
func Render(s Scene, w, h int) *Buffer {
	buf := NewBuffer(w, h)
	if w <= 0 || h <= 0 || len(s.Pos) == 0 || s.Cam == nil {
		return buf
	}

	pr := projectAll(s, w, h)
	foc := focusSet(s)

	drawEdges(buf, s, pr)
	drawNodes(buf, s, pr, foc)
	if s.ShowLabels {
		drawLabels(buf, s, pr)
	}
	drawHUD(buf, s)
	return buf
}

// focusSet marks the selected node and its direct (1-hop) neighbors. It
// drives focus-mode dimming and the highlighting of the selected node's
// edges; it is computed regardless of s.Focus so those edges always pop.
func focusSet(s Scene) []bool {
	f := make([]bool, len(s.Pos))
	if s.Selected < 0 || s.Selected >= len(f) {
		return f
	}
	f[s.Selected] = true
	for _, e := range s.Edges {
		a, b := e[0], e[1]
		if a == s.Selected && b >= 0 && b < len(f) {
			f[b] = true
		}
		if b == s.Selected && a >= 0 && a < len(f) {
			f[a] = true
		}
	}
	return f
}

// projectAll projects every node and fills in normalized depth and
// screen-center distance used for cueing, labeling and selection order.
func projectAll(s Scene, w, h int) []projected {
	pr := make([]projected, len(s.Pos))
	minD, maxD := math.Inf(1), math.Inf(-1)
	cx, cy := float64(w)/2, float64(h)/2
	for i, p := range s.Pos {
		x, y, depth, ok := s.Cam.Project(p, w, h)
		pr[i] = projected{id: i, x: x, y: y, depth: depth, visible: ok}
		if !ok {
			continue
		}
		if depth < minD {
			minD = depth
		}
		if depth > maxD {
			maxD = depth
		}
		dx, dy := float64(x)-cx, float64(y)-cy
		pr[i].centerDist = math.Hypot(dx, dy)
	}
	span := maxD - minD
	for i := range pr {
		if !pr[i].visible {
			pr[i].depthNorm = 1
			pr[i].centerDist = math.Inf(1)
			continue
		}
		if span > 1e-6 {
			pr[i].depthNorm = (pr[i].depth - minD) / span
		}
	}
	return pr
}

// slopeGlyph picks a box/diagonal drawing character matching an edge's
// screen direction, so a run of them reads as a continuous line rather than
// a scatter of dots. Terminal rows grow downward, so a positive dx/dy pair
// slopes down-right.
func slopeGlyph(x0, y0, x1, y1 int) rune {
	dx, dy := x1-x0, y1-y0
	adx, ady := abs(dx), abs(dy)
	switch {
	case adx >= 2*ady:
		return '─'
	case ady >= 2*adx:
		return '│'
	case (dx > 0) == (dy > 0):
		return '╲'
	default:
		return '╱'
	}
}

func drawEdges(buf *Buffer, s Scene, pr []projected) {
	type ei struct {
		a, b     int
		depth    float64
		incident bool // touches the selected node
	}
	edges := make([]ei, 0, len(s.Edges))
	for _, e := range s.Edges {
		a, b := e[0], e[1]
		if a < 0 || b < 0 || a >= len(pr) || b >= len(pr) || a == b {
			continue
		}
		if !pr[a].visible || !pr[b].visible {
			continue
		}
		// Every edge is drawn — the connecting lines are what tell you where
		// the nodes are. Focus mode changes emphasis, not visibility.
		incident := a == s.Selected || b == s.Selected
		edges = append(edges, ei{a, b, (pr[a].depth + pr[b].depth) / 2, incident})
	}
	// Painter's algorithm (far first), but always draw the selected node's
	// edges last so they overwrite the understated ones.
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].incident != edges[j].incident {
			return !edges[i].incident
		}
		return edges[i].depth > edges[j].depth
	})
	for _, e := range edges {
		glyph := slopeGlyph(pr[e.a].x, pr[e.a].y, pr[e.b].x, pr[e.b].y)
		// Understated mesh edges stay dim; nearer ones brighten a touch via
		// bold so depth still reads. The selected node's edges pop in accent.
		near := (pr[e.a].depthNorm+pr[e.b].depthNorm)/2 < 0.5
		fg := s.Dim
		bold := near
		prio := int8(1)
		if e.incident {
			fg = s.Accent
			bold = true
			prio = 3
		}
		line(pr[e.a].x, pr[e.a].y, pr[e.b].x, pr[e.b].y, func(x, y int) {
			buf.put(x, y, Cell{Ch: glyph, FG: fg, Bold: bold, prio: prio})
		})
	}
}

func drawNodes(buf *Buffer, s Scene, pr []projected, foc []bool) {
	order := make([]int, 0, len(pr))
	for i := range pr {
		if pr[i].visible {
			order = append(order, i)
		}
	}
	// Far nodes first so nearer nodes paint on top.
	sort.SliceStable(order, func(i, j int) bool { return pr[order[i]].depth > pr[order[j]].depth })
	for _, i := range order {
		p := pr[i]
		selected := i == s.Selected
		if s.Focus && !foc[i] {
			// Out-of-focus node: keep its real glyph (so its position stays
			// clear) but drain the color so the selected neighborhood is what
			// draws the eye.
			buf.put(p.x, p.y, Cell{Ch: nodeGlyph(p.depthNorm, degOf(s.Degrees, i), false), FG: s.Dim, prio: 5})
			continue
		}
		glyph := nodeGlyph(p.depthNorm, degOf(s.Degrees, i), selected)
		cell := Cell{Ch: glyph, prio: 5}
		if selected {
			cell.FG = s.Accent
			cell.Bold = true
			cell.Inv = true
			cell.prio = 9
		} else {
			cell.FG = nodeColor(baseOf(s.Bases, i), s.Dim, p.depthNorm)
			cell.Bold = p.depthNorm < 0.35
		}
		buf.put(p.x, p.y, cell)
	}
}

// drawLabels titles every visible node (toggle off with L). Proximity stands
// in for font size: near titles are bold and shown in full, far ones are dim
// and clipped short. Nearest nodes are placed first so they win contested
// cells; overlap avoidance then drops only the farther labels.
func drawLabels(buf *Buffer, s Scene, pr []projected) {
	order := make([]int, 0, len(pr))
	for i := range pr {
		if pr[i].visible {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return pr[order[i]].depth < pr[order[j]].depth })

	seen := map[int]bool{}
	draw := func(id int, force bool) {
		if seen[id] || id < 0 || id >= len(pr) || !pr[id].visible {
			return
		}
		seen[id] = true
		p := pr[id]
		selected := id == s.Selected
		// Proximity tiers approximate size: closer = fuller + bold.
		maxw, bold := 12, false
		switch {
		case selected || p.depthNorm < 0.34:
			maxw, bold = 30, true
		case p.depthNorm < 0.67:
			maxw, bold = 20, false
		}
		title := truncate(titleOf(s.Titles, id), maxw)
		if title == "" {
			return
		}
		fg := s.Dim
		if selected {
			fg = s.Accent
		} else if p.depthNorm < 0.67 {
			fg = nodeColor(baseOf(s.Bases, id), s.Dim, p.depthNorm)
		}
		placeLabel(buf, p.x+1, p.y, title, fg, bold, force)
	}
	// The selected node is always labeled, even over a contested cell.
	draw(s.Selected, true)
	for _, id := range order {
		draw(id, false)
	}
}

// placeLabel writes text starting at (x,y), honoring double-width runes so
// the buffer's visible width is unchanged. Unless force is set it yields to
// cells already holding a node or another label (prio >= 4) but will draw
// over the muted edge mesh, so titles stay visible without clobbering nodes.
func placeLabel(buf *Buffer, x, y int, text, fg string, bold, force bool) {
	if y < 0 || y >= buf.H {
		return
	}
	runes := []rune(text)
	// Reserve a leading space so labels don't butt against their node.
	cells := make([][2]int, 0, len(runes)+1)
	col := x
	widths := make([]int, 0, len(runes)+1)
	glyphs := make([]rune, 0, len(runes)+1)
	glyphs = append(glyphs, ' ')
	widths = append(widths, 1)
	for _, r := range runes {
		glyphs = append(glyphs, r)
		widths = append(widths, cellWidth(r))
	}
	for i := range glyphs {
		w := widths[i]
		if col < 0 || col+w > buf.W {
			return // would overflow; skip whole label (all-or-nothing)
		}
		cells = append(cells, [2]int{col, y})
		if w == 2 {
			cells = append(cells, [2]int{col + 1, y})
		}
		col += w
	}
	// Overlap check: bail only if a target cell holds a node or another
	// label; edge-mesh cells (lower priority) may be drawn over.
	if !force {
		for _, c := range cells {
			if occ := buf.At(c[0], c[1]); occ.set && occ.prio >= 4 {
				return
			}
		}
	}
	col = x
	for i, g := range glyphs {
		w := widths[i]
		buf.put(col, y, Cell{Ch: g, FG: fg, Bold: bold, prio: 4})
		if w == 2 {
			buf.put(col+1, y, Cell{Ch: 0, FG: fg, Bold: bold, prio: 4})
		}
		col += w
	}
}

// drawHUD writes a one-line status in the top-left: selected title, its link
// degree, and node/edge counts.
func drawHUD(buf *Buffer, s Scene) {
	if buf.H == 0 {
		return
	}
	title := "—"
	deg := 0
	if s.Selected >= 0 && s.Selected < len(s.Titles) {
		title = truncate(titleOf(s.Titles, s.Selected), 28)
		deg = degOf(s.Degrees, s.Selected)
	}
	hud := " " + title + "  " + plural(deg, "link") + "  " +
		plural(len(s.Pos), "node") + " / " + plural(len(s.Edges), "edge")
	if s.Focus {
		hud += "  ⊙focus"
	}
	hud += " "
	writeRaw(buf, 0, 0, hud, s.Accent, true)
}

// writeRaw stamps text at (x,y) overwriting whatever is there (used for the
// HUD, which sits above everything). Honors double-width runes.
func writeRaw(buf *Buffer, x, y int, text, fg string, bold bool) {
	col := x
	for _, r := range []rune(text) {
		w := cellWidth(r)
		if col+w > buf.W {
			return
		}
		buf.put(col, y, Cell{Ch: r, FG: fg, Bold: bold, prio: 20})
		if w == 2 {
			buf.put(col+1, y, Cell{Ch: 0, FG: fg, Bold: bold, prio: 20})
		}
		col += w
	}
}

// nodeGlyph picks a node glyph by depth (and hub-ness for near nodes): near
// hubs are the boldest.
func nodeGlyph(depthNorm float64, degree int, selected bool) rune {
	if selected {
		return '◉'
	}
	switch {
	case depthNorm < 0.25:
		if degree >= 4 {
			return '◉'
		}
		return '●'
	case depthNorm < 0.55:
		return '●'
	case depthNorm < 0.8:
		return '○'
	default:
		return '·'
	}
}

// nodeColor tints a node by its base (stable hash into a pleasant 256-color
// subset), fading toward the dim color as depth grows.
func nodeColor(base, dim string, depthNorm float64) string {
	if depthNorm > 0.75 {
		return dim
	}
	return baseColor(base)
}

// pleasantColors is a hand-picked subset of the xterm-256 palette that reads
// well on both light and dark terminals; base names hash into it.
var pleasantColors = []string{
	"39", "43", "78", "108", "141", "168", "179", "110",
	"114", "147", "180", "210", "115", "152", "218", "153",
}

// baseColor maps a base name to a stable color from pleasantColors.
func baseColor(base string) string {
	h := fnv.New32a()
	h.Write([]byte(base))
	return pleasantColors[int(h.Sum32())%len(pleasantColors)]
}

// line rasterizes a Bresenham line from (x0,y0) to (x1,y1), invoking plot at
// each cell. Endpoints are included; callers clip via the buffer.
func line(x0, y0, x1, y1 int, plot func(x, y int)) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	// Guard against pathological far-offscreen coordinates.
	const maxSteps = 100000
	for steps := 0; steps < maxSteps; steps++ {
		plot(x0, y0)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// truncate shortens s to at most max display columns, appending '…' when it
// cuts (accounting for double-width runes).
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	col := 0
	var b strings.Builder
	for _, r := range s {
		w := cellWidth(r)
		if col+w > max-1 {
			b.WriteRune('…')
			return b.String()
		}
		b.WriteRune(r)
		col += w
	}
	return b.String()
}

func plural(n int, unit string) string {
	s := itoa(n) + " " + unit
	if n != 1 {
		s += "s"
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func titleOf(t []string, i int) string {
	if i >= 0 && i < len(t) {
		return t[i]
	}
	return ""
}
func degOf(d []int, i int) int {
	if i >= 0 && i < len(d) {
		return d[i]
	}
	return 0
}
func baseOf(b []string, i int) string {
	if i >= 0 && i < len(b) {
		return b[i]
	}
	return ""
}

// cellWidth returns the terminal column width of a rune: 0 for combining
// marks, 2 for common wide (CJK/full-width/emoji) ranges, else 1. It is a
// deliberately small approximation so the renderer can guarantee exact line
// widths without pulling in an external width dependency.
func cellWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0 // control
	}
	// Combining marks.
	if (r >= 0x0300 && r <= 0x036f) || (r >= 0x1ab0 && r <= 0x1aff) ||
		(r >= 0x1dc0 && r <= 0x1dff) || (r >= 0x20d0 && r <= 0x20ff) ||
		(r >= 0xfe20 && r <= 0xfe2f) {
		return 0
	}
	if isWide(r) {
		return 2
	}
	return 1
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f, // Hangul Jamo
		r >= 0x2e80 && r <= 0x303e, // CJK radicals, Kangxi
		r >= 0x3041 && r <= 0x33ff, // Hiragana..CJK symbols
		r >= 0x3400 && r <= 0x4dbf, // CJK ext A
		r >= 0x4e00 && r <= 0x9fff, // CJK unified
		r >= 0xa000 && r <= 0xa4cf, // Yi
		r >= 0xac00 && r <= 0xd7a3, // Hangul syllables
		r >= 0xf900 && r <= 0xfaff, // CJK compat
		r >= 0xfe30 && r <= 0xfe4f, // CJK compat forms
		r >= 0xff00 && r <= 0xff60, // fullwidth forms
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1faff, // emoji / symbols
		r >= 0x20000 && r <= 0x3fffd: // CJK ext B+
		return true
	}
	return false
}
