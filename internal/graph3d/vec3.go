// Package graph3d holds the pure, terminal-agnostic math and logic behind
// nepenthe's 3D link-graph view: vector math, a force-directed layout, an
// orbiting perspective camera, and a cell-buffer renderer. It has no
// dependency on Bubble Tea or the vault package so it can be exercised in
// isolation by unit tests.
package graph3d

import "math"

// Vec3 is a point or direction in layout space (float64 precision).
type Vec3 struct {
	X, Y, Z float64
}

// Add returns a+b.
func (a Vec3) Add(b Vec3) Vec3 { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }

// Sub returns a-b.
func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }

// Scale returns a scaled by s.
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }

// Dot returns the dot product a·b.
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

// Length returns the Euclidean magnitude of a.
func (a Vec3) Length() float64 { return math.Sqrt(a.Dot(a)) }

// Length2 returns the squared magnitude of a (cheaper than Length).
func (a Vec3) Length2() float64 { return a.Dot(a) }

// Normalize returns a unit vector in the direction of a. The zero vector
// is returned unchanged so callers never divide by zero.
func (a Vec3) Normalize() Vec3 {
	l := a.Length()
	if l < 1e-12 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}
