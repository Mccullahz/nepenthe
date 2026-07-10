package graph3d

import "testing"

func sampleGraph() ([]string, [][2]int) {
	paths := []string{"a.md", "b.md", "c.md", "d.md", "e.md"}
	edges := [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 0}, {0, 2}}
	return paths, edges
}

func TestLayoutDeterministic(t *testing.T) {
	paths, edges := sampleGraph()
	p := Params{}
	l1 := NewLayout(paths, nil, edges, p, nil)
	l2 := NewLayout(paths, nil, edges, p, nil)
	l1.Step(200)
	l2.Step(200)
	for i := range l1.Positions() {
		if l1.Positions()[i] != l2.Positions()[i] {
			t.Fatalf("node %d differs: %v vs %v", i, l1.Positions()[i], l2.Positions()[i])
		}
	}
}

func TestEnergyDecreases(t *testing.T) {
	paths, edges := sampleGraph()
	l := NewLayout(paths, nil, edges, Params{}, nil)
	l.Step(30)
	early := l.Energy()
	l.Step(400)
	late := l.Energy()
	if late >= early {
		t.Fatalf("energy did not decrease: early=%g late=%g", early, late)
	}
	if late > energyEps {
		// Not a hard failure boundary but the layout should be near rest.
		t.Logf("energy after settling: %g", late)
	}
}

const energyEps = 1.0

func TestPreservePositionsAcrossRebuild(t *testing.T) {
	paths, edges := sampleGraph()
	l := NewLayout(paths, nil, edges, Params{}, nil)
	l.Step(100)
	prev := l.PositionMap()

	// Rebuild with one extra node; surviving nodes keep their positions.
	paths2 := append([]string{}, paths...)
	paths2 = append(paths2, "f.md")
	edges2 := append([][2]int{}, edges...)
	edges2 = append(edges2, [2]int{5, 0})
	l2 := NewLayout(paths2, nil, edges2, Params{}, prev)
	for i := 0; i < len(paths); i++ {
		if l2.Positions()[i] != prev[paths[i]] {
			t.Fatalf("node %d not preserved: %v vs %v", i, l2.Positions()[i], prev[paths[i]])
		}
	}
	// The new node should be placed near its neighbor (node 0), not at origin.
	fPos := l2.Positions()[5]
	if fPos == (Vec3{}) {
		t.Fatalf("new node placed at origin")
	}
}

func TestBoundingRadiusPositive(t *testing.T) {
	paths, edges := sampleGraph()
	l := NewLayout(paths, nil, edges, Params{}, nil)
	l.Step(100)
	if r := l.BoundingRadius(); r <= 0 {
		t.Fatalf("bounding radius = %g", r)
	}
}

func TestSingleNodeStable(t *testing.T) {
	l := NewLayout([]string{"only.md"}, nil, nil, Params{}, nil)
	l.Step(50)
	if e := l.Energy(); e != 0 {
		t.Fatalf("single node energy = %g, want 0", e)
	}
}

func TestClusteringSeparatesBases(t *testing.T) {
	// Three bases, several nodes each, with a couple of cross-base edges.
	var paths, bases []string
	add := func(base string, k int) {
		for i := 0; i < k; i++ {
			paths = append(paths, base+"/n"+string(rune('0'+i))+".md")
			bases = append(bases, base)
		}
	}
	add("alpha", 6)
	add("beta", 6)
	add("gamma", 6)
	edges := [][2]int{{0, 1}, {6, 7}, {12, 13}, {0, 6}, {6, 12}} // some intra + cross links
	l := NewLayout(paths, bases, edges, Params{LinkDistance: 3, Repulsion: 6, Cluster: true}, nil)
	for i := 0; i < 600; i++ {
		l.Step(1)
	}
	pos := l.Positions()

	// Per-base centroid and the max distance of a member from it (cluster
	// radius), plus the min distance between distinct base centroids.
	centroid := map[string]Vec3{}
	count := map[string]int{}
	for i, b := range bases {
		centroid[b] = centroid[b].Add(pos[i])
		count[b]++
	}
	for b := range centroid {
		centroid[b] = centroid[b].Scale(1 / float64(count[b]))
	}
	maxIntra := 0.0
	for i, b := range bases {
		if d := pos[i].Sub(centroid[b]).Length(); d > maxIntra {
			maxIntra = d
		}
	}
	names := []string{"alpha", "beta", "gamma"}
	minInter := 1e18
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if d := centroid[names[i]].Sub(centroid[names[j]]).Length(); d < minInter {
				minInter = d
			}
		}
	}
	// Clusters must be clearly separated: the nearest two base centers are
	// farther apart than the widest cluster's diameter.
	if minInter <= 2*maxIntra {
		t.Errorf("bases not separated: minInterCentroid=%.2f, maxIntraRadius=%.2f (want minInter > 2*maxIntra)", minInter, maxIntra)
	}
}

func TestClusteringOffKeepsSingleBlob(t *testing.T) {
	// With Cluster off, the base labels are ignored (one blob), matching the
	// original behavior.
	paths := []string{"a/x.md", "b/y.md"}
	bases := []string{"a", "b"}
	l := NewLayout(paths, bases, nil, Params{LinkDistance: 3, Repulsion: 6, Cluster: false}, nil)
	if l.clustering {
		t.Error("clustering should be off")
	}
	for i := range l.baseCenter {
		if l.baseCenter[i] != (Vec3{}) {
			t.Errorf("baseCenter[%d] = %v, want origin when not clustering", i, l.baseCenter[i])
		}
	}
}
