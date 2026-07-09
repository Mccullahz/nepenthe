package graph3d

import (
	"math"
	"testing"
)

func TestVec3Ops(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, -1, 0}
	if got := a.Add(b); got != (Vec3{5, 1, 3}) {
		t.Errorf("Add = %v", got)
	}
	if got := a.Sub(b); got != (Vec3{-3, 3, 3}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Scale(2); got != (Vec3{2, 4, 6}) {
		t.Errorf("Scale = %v", got)
	}
	if got := a.Dot(b); got != 2 {
		t.Errorf("Dot = %v, want 2", got)
	}
	if got := (Vec3{3, 4, 0}).Length(); got != 5 {
		t.Errorf("Length = %v, want 5", got)
	}
}

func TestNormalize(t *testing.T) {
	n := Vec3{0, 3, 4}.Normalize()
	if math.Abs(n.Length()-1) > 1e-9 {
		t.Errorf("Normalize length = %v, want 1", n.Length())
	}
	// Zero vector is returned unchanged (no NaN).
	if z := (Vec3{}).Normalize(); z != (Vec3{}) {
		t.Errorf("Normalize(0) = %v, want zero", z)
	}
}
