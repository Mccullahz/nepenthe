package graph3d

import (
	"hash/fnv"
	"math"
	"math/rand"
)

// Params tunes the force-directed layout. Zero values are replaced with
// sensible defaults by DefaultParams-style normalization in NewLayout.
type Params struct {
	LinkDistance float64 // preferred edge length
	Repulsion    float64 // node-node repulsion strength
	Gravity      float64 // pull toward the origin (keeps the graph centered)
	Damping      float64 // velocity retained each step (0..1)
	TimeStep     float64 // integration step
	MaxSpeed     float64 // per-node velocity clamp (stability)
	// Cluster groups nodes by base into separate regions of space, each
	// base pulled toward its own spread-out center instead of the origin.
	Cluster bool
}

func (p Params) normalized() Params {
	if p.LinkDistance <= 0 {
		p.LinkDistance = 3.0
	}
	if p.Repulsion <= 0 {
		p.Repulsion = 6.0
	}
	if p.Gravity <= 0 {
		p.Gravity = 0.02
	}
	if p.Damping <= 0 || p.Damping >= 1 {
		p.Damping = 0.85
	}
	if p.TimeStep <= 0 {
		p.TimeStep = 0.18
	}
	if p.MaxSpeed <= 0 {
		p.MaxSpeed = 4.0
	}
	return p
}

// sampleThreshold is the node count above which pairwise O(n²) repulsion is
// replaced by deterministic random sampling to keep each Step cheap.
const sampleThreshold = 1500

// clusterPull is the gravity strength toward a base's cluster center when
// clustering is on (stronger than the default origin gravity so clusters
// stay tight and separated).
const clusterPull = 0.09

// clusterRadius estimates the resting radius of a base's cluster of k nodes,
// used both to seed members and to space base centers so clusters don't
// overlap.
func clusterRadius(linkDist float64, k int) float64 {
	if k < 1 {
		k = 1
	}
	return linkDist * math.Cbrt(float64(k)) * 1.3
}

// Layout is a force-directed 3D placement of graph nodes. Positions are
// deterministic for a given set of node paths and edges: initial placement
// is a Fibonacci sphere jittered by a hash of each node's path, so a rescan
// that returns the same graph reproduces the same layout.
type Layout struct {
	pos    []Vec3
	vel    []Vec3
	edges  [][2]int
	paths  []string
	params Params

	// baseCenter[i] is the anchor each node's gravity pulls toward — its
	// base's cluster center when clustering, else the origin (zero).
	baseCenter []Vec3
	clustering bool

	energy float64 // kinetic energy after the most recent Step
	step   int     // total iterations advanced (seeds sampling PRNG)
}

// NewLayout builds a layout for the given node paths and edges (edges are
// index pairs into paths). bases[i] is node i's base (same length as paths;
// nil or a mismatched length means a single unclustered base). prev, if
// non-nil, maps a node path to a position to reuse, so nodes surviving a
// rescan keep their place; new nodes are seeded near their base center (or a
// linked neighbor) when possible, else on the jittered sphere.
func NewLayout(paths, bases []string, edges [][2]int, params Params, prev map[string]Vec3) *Layout {
	n := len(paths)
	l := &Layout{
		pos:    make([]Vec3, n),
		vel:    make([]Vec3, n),
		edges:  edges,
		paths:  paths,
		params: params.normalized(),
	}
	l.baseCenter = make([]Vec3, n) // zero (origin) unless clustering assigns

	// Assign each base a stable ordinal and count its members. Bases appear
	// in first-seen order (deterministic given the caller's node order).
	if len(bases) != n {
		bases = make([]string, n) // one empty base -> no clustering
	}
	baseIdx := make([]int, n)
	seen := make(map[string]int)
	var order []string
	for i, b := range bases {
		bi, ok := seen[b]
		if !ok {
			bi = len(order)
			seen[b] = bi
			order = append(order, b)
		}
		baseIdx[i] = bi
	}
	nb := len(order)
	counts := make([]int, nb)
	local := make([]int, n) // index of node within its base
	for i := range paths {
		local[i] = counts[baseIdx[i]]
		counts[baseIdx[i]]++
	}
	l.clustering = params.Cluster && nb > 1

	// Spread the base centers on a sphere sized so clusters don't overlap.
	centers := make([]Vec3, nb) // origin when not clustering
	if l.clustering {
		maxCR := 0.0
		for bi := 0; bi < nb; bi++ {
			if cr := clusterRadius(l.params.LinkDistance, counts[bi]); cr > maxCR {
				maxCR = cr
			}
		}
		// Desired center-to-center spacing, then the sphere radius that
		// yields it for nb points (nearest-neighbor spacing ~ 2R/sqrt(nb)).
		spacing := 2*maxCR + l.params.LinkDistance*3
		sep := spacing * math.Sqrt(float64(nb)) / 2
		for bi := 0; bi < nb; bi++ {
			centers[bi] = fibonacciSphere(bi, nb, sep)
		}
	}
	for i := range paths {
		l.baseCenter[i] = centers[baseIdx[i]]
	}

	globalRadius := l.params.LinkDistance * math.Cbrt(float64(n)+1) * 1.6
	placed := make([]bool, n)
	for i, p := range paths {
		if prev != nil {
			if v, ok := prev[p]; ok {
				l.pos[i] = v
				placed[i] = true
				continue
			}
		}
		if l.clustering {
			// Seed on a small sphere around the node's base center.
			cr := clusterRadius(l.params.LinkDistance, counts[baseIdx[i]])
			l.pos[i] = l.baseCenter[i].
				Add(fibonacciSphere(local[i], counts[baseIdx[i]], cr)).
				Add(hashJitter(p, l.params.LinkDistance))
		} else {
			l.pos[i] = fibonacciSphere(i, n, globalRadius).Add(hashJitter(p, l.params.LinkDistance))
		}
	}
	// Pull brand-new nodes toward an already-placed neighbor so they enter
	// the layout near where they belong instead of flinging it apart.
	if prev != nil {
		neighbor := make([]int, n)
		for i := range neighbor {
			neighbor[i] = -1
		}
		for _, e := range edges {
			a, b := e[0], e[1]
			if placed[a] && !placed[b] && neighbor[b] < 0 {
				neighbor[b] = a
			}
			if placed[b] && !placed[a] && neighbor[a] < 0 {
				neighbor[a] = b
			}
		}
		for i := range paths {
			if !placed[i] && neighbor[i] >= 0 {
				l.pos[i] = l.pos[neighbor[i]].Add(hashJitter(paths[i], l.params.LinkDistance))
			}
		}
	}
	return l
}

// Positions returns the live position slice (do not mutate).
func (l *Layout) Positions() []Vec3 { return l.pos }

// Energy returns the total kinetic energy after the most recent Step. It
// trends toward zero as the layout settles; callers use it to decide when
// to stop animating.
func (l *Layout) Energy() float64 { return l.energy }

// Len returns the number of nodes.
func (l *Layout) Len() int { return len(l.pos) }

// Step advances the simulation by iters iterations and returns the final
// kinetic energy. Work per iteration is O(n²) up to sampleThreshold nodes
// and O(n·k) above it via deterministic sampling, so a frame stays cheap.
func (l *Layout) Step(iters int) float64 {
	n := len(l.pos)
	if n < 2 {
		l.energy = 0
		return 0
	}
	p := l.params
	force := make([]Vec3, n)
	for it := 0; it < iters; it++ {
		for i := range force {
			force[i] = Vec3{}
		}
		l.repulsion(force)
		// Spring attraction along edges toward LinkDistance.
		for _, e := range l.edges {
			a, b := e[0], e[1]
			if a == b || a < 0 || b < 0 || a >= n || b >= n {
				continue
			}
			d := l.pos[b].Sub(l.pos[a])
			dist := d.Length()
			if dist < 1e-6 {
				continue
			}
			f := 0.1 * (dist - p.LinkDistance)
			dir := d.Scale(1 / dist)
			force[a] = force[a].Add(dir.Scale(f))
			force[b] = force[b].Sub(dir.Scale(f))
		}
		// Gravity pulls each node toward its anchor: its base's cluster
		// center when clustering (which both groups and keeps bases apart),
		// else the origin. The cluster pull is stronger so clusters stay
		// tight and distinct against inter-cluster repulsion.
		pull := p.Gravity
		if l.clustering {
			pull = clusterPull
		}
		for i := range l.pos {
			force[i] = force[i].Sub(l.pos[i].Sub(l.baseCenter[i]).Scale(pull))
		}
		// Integrate with damping and a speed clamp.
		var energy float64
		for i := range l.pos {
			v := l.vel[i].Add(force[i].Scale(p.TimeStep)).Scale(p.Damping)
			if sp := v.Length(); sp > p.MaxSpeed {
				v = v.Scale(p.MaxSpeed / sp)
			}
			l.vel[i] = v
			l.pos[i] = l.pos[i].Add(v.Scale(p.TimeStep))
			energy += v.Length2()
		}
		l.energy = energy
		l.step++
	}
	return l.energy
}

// repulsion accumulates node-node repulsion into force. Below the sample
// threshold every pair is considered; above it each node repels a fixed,
// deterministically chosen sample of others (scaled to approximate the full
// sum) so the cost stays linear for very large graphs.
func (l *Layout) repulsion(force []Vec3) {
	n := len(l.pos)
	if n <= sampleThreshold {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				l.repel(force, i, j, 1)
			}
		}
		return
	}
	const k = 64
	scale := float64(n-1) / float64(k)
	rng := rand.New(rand.NewSource(int64(l.step)*2654435761 + 1))
	for i := 0; i < n; i++ {
		for s := 0; s < k; s++ {
			j := rng.Intn(n)
			if j == i {
				continue
			}
			l.repel(force, i, j, scale)
		}
	}
}

// repel applies an inverse-square repulsion between nodes i and j, weighted
// by w (used to rescale sampled sums).
func (l *Layout) repel(force []Vec3, i, j int, w float64) {
	d := l.pos[i].Sub(l.pos[j])
	dist2 := d.Length2()
	if dist2 < 0.01 {
		dist2 = 0.01
	}
	dist := math.Sqrt(dist2)
	f := w * l.params.Repulsion / dist2
	dir := d.Scale(1 / dist)
	force[i] = force[i].Add(dir.Scale(f))
	force[j] = force[j].Sub(dir.Scale(f))
}

// Centroid returns the mean position of all nodes.
func (l *Layout) Centroid() Vec3 {
	if len(l.pos) == 0 {
		return Vec3{}
	}
	var c Vec3
	for _, p := range l.pos {
		c = c.Add(p)
	}
	return c.Scale(1 / float64(len(l.pos)))
}

// BoundingRadius returns the distance from the centroid to the farthest
// node, i.e. the radius of the sphere that encloses the whole graph.
func (l *Layout) BoundingRadius() float64 {
	c := l.Centroid()
	var r float64
	for _, p := range l.pos {
		if d := p.Sub(c).Length(); d > r {
			r = d
		}
	}
	if r < 1 {
		r = 1
	}
	return r
}

// PositionMap returns a snapshot of path->position for reuse across rescans.
func (l *Layout) PositionMap() map[string]Vec3 {
	m := make(map[string]Vec3, len(l.paths))
	for i, p := range l.paths {
		m[p] = l.pos[i]
	}
	return m
}

// fibonacciSphere returns the i-th of n points evenly spread on a sphere of
// the given radius (the classic Fibonacci spiral), a stable, well-distributed
// seed for the layout.
func fibonacciSphere(i, n int, radius float64) Vec3 {
	if n <= 1 {
		return Vec3{}
	}
	phi := math.Acos(1 - 2*(float64(i)+0.5)/float64(n))
	golden := math.Pi * (1 + math.Sqrt(5))
	theta := golden * float64(i)
	return Vec3{
		X: math.Sin(phi) * math.Cos(theta) * radius,
		Y: math.Sin(phi) * math.Sin(theta) * radius,
		Z: math.Cos(phi) * radius,
	}
}

// hashJitter derives a small deterministic offset from a node path so two
// nodes never start at the exact same coordinate (which would make repulsion
// degenerate) while keeping layouts reproducible across runs.
func hashJitter(path string, scale float64) Vec3 {
	h := fnv.New64a()
	h.Write([]byte(path))
	v := h.Sum64()
	f := func(shift uint) float64 {
		// map a byte to [-0.5, 0.5]
		return (float64((v>>shift)&0xff)/255.0 - 0.5)
	}
	return Vec3{f(0), f(8), f(16)}.Scale(scale * 0.5)
}
