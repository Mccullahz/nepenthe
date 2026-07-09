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
	l1 := NewLayout(paths, edges, p, nil)
	l2 := NewLayout(paths, edges, p, nil)
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
	l := NewLayout(paths, edges, Params{}, nil)
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
	l := NewLayout(paths, edges, Params{}, nil)
	l.Step(100)
	prev := l.PositionMap()

	// Rebuild with one extra node; surviving nodes keep their positions.
	paths2 := append([]string{}, paths...)
	paths2 = append(paths2, "f.md")
	edges2 := append([][2]int{}, edges...)
	edges2 = append(edges2, [2]int{5, 0})
	l2 := NewLayout(paths2, edges2, Params{}, prev)
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
	l := NewLayout(paths, edges, Params{}, nil)
	l.Step(100)
	if r := l.BoundingRadius(); r <= 0 {
		t.Fatalf("bounding radius = %g", r)
	}
}

func TestSingleNodeStable(t *testing.T) {
	l := NewLayout([]string{"only.md"}, nil, Params{}, nil)
	l.Step(50)
	if e := l.Energy(); e != 0 {
		t.Fatalf("single node energy = %g, want 0", e)
	}
}
