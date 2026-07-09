package graph3d

import "math"

// aspectX corrects for terminal cells being roughly twice as tall as they
// are wide: horizontal screen coordinates are stretched by this factor so a
// sphere orbits as a sphere instead of an egg.
const aspectX = 2.0

// pitchLimit clamps the camera's elevation just short of the poles to avoid
// gimbal flip and a degenerate up-vector.
const pitchLimit = math.Pi/2 - 0.05

// Camera is an orbiting perspective camera. It looks at Center from a point
// determined by (Yaw, Pitch, Dist). Target* fields are eased toward by
// EaseStep so key taps animate smoothly rather than snapping.
type Camera struct {
	Yaw, Pitch, Dist float64
	TargetYaw        float64
	TargetPitch      float64
	TargetDist       float64
	Center           Vec3
	TargetCenter     Vec3
	FOV              float64 // vertical field of view, degrees

	MinDist, MaxDist float64
}

// NewCamera returns a camera with sane defaults and the given field of view.
func NewCamera(fov float64) *Camera {
	if fov <= 0 || fov >= 179 {
		fov = 70
	}
	c := &Camera{
		Yaw:         0.6,
		Pitch:       0.35,
		Dist:        20,
		TargetYaw:   0.6,
		TargetPitch: 0.35,
		TargetDist:  20,
		FOV:         fov,
		MinDist:     1.5,
		MaxDist:     400,
	}
	return c
}

// OrbitBy nudges the target yaw/pitch (radians). Pitch is clamped short of
// the poles.
func (c *Camera) OrbitBy(dYaw, dPitch float64) {
	c.TargetYaw += dYaw
	c.TargetPitch = clamp(c.TargetPitch+dPitch, -pitchLimit, pitchLimit)
}

// ZoomBy scales the target distance by factor (factor<1 zooms in), clamped
// to [MinDist, MaxDist].
func (c *Camera) ZoomBy(factor float64) {
	c.TargetDist = clamp(c.TargetDist*factor, c.MinDist, c.MaxDist)
}

// LookAt pans the camera to orbit around target without changing the zoom
// distance or orbit angles. Selection cycling uses it to keep the chosen
// node centered and readable.
func (c *Camera) LookAt(target Vec3) { c.TargetCenter = target }

// Frame points the camera at center and sets the target distance so a sphere
// of the given radius fits the w×h viewport (accounting for the aspect
// stretch), then keeps the current orbit angles.
func (c *Camera) Frame(center Vec3, radius, w, h float64) {
	c.TargetCenter = center
	f := focal(c.FOV)
	// Vertical fit: f*radius/dist <= 1. Horizontal fit with the aspectX
	// stretch: f*radius/dist * aspectX <= w/h. Take the tighter of the two.
	need := f * radius
	if h > 0 && w > 0 {
		if hz := f * radius * aspectX * h / w; hz > need {
			need = hz
		}
	}
	dist := need * 1.35
	c.MaxDist = math.Max(c.MaxDist, dist*3)
	c.TargetDist = clamp(dist, c.MinDist, c.MaxDist)
}

// EaseStep moves the current pose a fraction toward the target and reports
// whether meaningful motion remains (so the caller keeps animating).
func (c *Camera) EaseStep() bool {
	const rate = 0.25
	moving := false
	ease := func(cur *float64, target float64, eps float64) {
		d := target - *cur
		if math.Abs(d) < eps {
			*cur = target
			return
		}
		*cur += d * rate
		moving = true
	}
	ease(&c.Yaw, c.TargetYaw, 1e-4)
	ease(&c.Pitch, c.TargetPitch, 1e-4)
	ease(&c.Dist, c.TargetDist, 1e-3)
	ease(&c.Center.X, c.TargetCenter.X, 1e-3)
	ease(&c.Center.Y, c.TargetCenter.Y, 1e-3)
	ease(&c.Center.Z, c.TargetCenter.Z, 1e-3)
	return moving
}

// Project maps a world point to integer cell coordinates in a w×h buffer,
// returning the depth (distance in front of the camera) and whether the
// point is in front of the camera. Horizontal coordinates include the
// aspectX stretch so orbits look spherical.
func (c *Camera) Project(p Vec3, w, h int) (sx, sy int, depth float64, ok bool) {
	x, y, z := c.viewSpace(p)
	depth = c.Dist - z
	if depth <= 1e-3 {
		return 0, 0, depth, false
	}
	f := focal(c.FOV)
	ndcx := f * x / depth
	ndcy := f * y / depth
	scale := float64(h) / 2
	fx := float64(w)/2 + ndcx*scale*aspectX
	fy := float64(h)/2 - ndcy*scale
	return int(math.Round(fx)), int(math.Round(fy)), depth, true
}

// viewSpace rotates a world point into camera space (before perspective):
// translate by -Center, yaw about Y, then pitch about X.
func (c *Camera) viewSpace(p Vec3) (x, y, z float64) {
	t := p.Sub(c.Center)
	cy, sy := math.Cos(c.Yaw), math.Sin(c.Yaw)
	x1 := t.X*cy + t.Z*sy
	z1 := -t.X*sy + t.Z*cy
	y1 := t.Y
	cp, sp := math.Cos(c.Pitch), math.Sin(c.Pitch)
	y2 := y1*cp - z1*sp
	z2 := y1*sp + z1*cp
	return x1, y2, z2
}

// focal returns the perspective focal length 1/tan(fov/2) for a vertical
// field of view in degrees.
func focal(fovDeg float64) float64 {
	half := fovDeg * math.Pi / 360
	t := math.Tan(half)
	if t < 1e-6 {
		t = 1e-6
	}
	return 1 / t
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
