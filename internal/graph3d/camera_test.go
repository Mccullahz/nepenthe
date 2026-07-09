package graph3d

import (
	"math"
	"testing"
)

func TestProjectCentered(t *testing.T) {
	c := NewCamera(70)
	c.Yaw, c.Pitch, c.Dist = 0, 0, 20
	c.Center = Vec3{}
	// A point at the center projects to the middle of the buffer.
	x, y, depth, ok := c.Project(Vec3{}, 100, 40)
	if !ok {
		t.Fatal("center point not visible")
	}
	if x != 50 || y != 20 {
		t.Errorf("center projected to (%d,%d), want (50,20)", x, y)
	}
	if math.Abs(depth-20) > 1e-9 {
		t.Errorf("center depth = %g, want 20", depth)
	}
}

func TestAspectCorrection(t *testing.T) {
	c := NewCamera(70)
	c.Yaw, c.Pitch, c.Dist = 0, 0, 20
	// Equal world offsets in +X and +Y should project to roughly twice as
	// many horizontal cells as vertical, correcting the 2:1 cell aspect so
	// the shape reads as round.
	xr, _, _, _ := c.Project(Vec3{1, 0, 0}, 200, 200)
	_, yr, _, _ := c.Project(Vec3{0, 1, 0}, 200, 200)
	dx := float64(xr - 100)
	dy := float64(100 - yr)
	ratio := dx / dy
	if math.Abs(ratio-2) > 0.05 {
		t.Errorf("aspect ratio dx/dy = %g, want ~2", ratio)
	}
}

func TestDepthOrdering(t *testing.T) {
	c := NewCamera(70)
	c.Yaw, c.Pitch, c.Dist = 0, 0, 20
	// A point with larger +Z is closer to the camera (smaller depth).
	_, _, near, _ := c.Project(Vec3{0, 0, 5}, 100, 40)
	_, _, far, _ := c.Project(Vec3{0, 0, -5}, 100, 40)
	if !(near < far) {
		t.Errorf("expected near(%g) < far(%g)", near, far)
	}
}

func TestBehindCameraNotVisible(t *testing.T) {
	c := NewCamera(70)
	c.Yaw, c.Pitch, c.Dist = 0, 0, 5
	// A point far behind the camera plane is culled.
	if _, _, _, ok := c.Project(Vec3{0, 0, 10}, 100, 40); ok {
		t.Error("point behind camera reported visible")
	}
}

func TestPitchClamp(t *testing.T) {
	c := NewCamera(70)
	for i := 0; i < 100; i++ {
		c.OrbitBy(0, 1)
	}
	if c.TargetPitch > pitchLimit+1e-9 {
		t.Errorf("pitch %g exceeded limit %g", c.TargetPitch, pitchLimit)
	}
	for i := 0; i < 200; i++ {
		c.OrbitBy(0, -1)
	}
	if c.TargetPitch < -pitchLimit-1e-9 {
		t.Errorf("pitch %g below -limit %g", c.TargetPitch, -pitchLimit)
	}
}

func TestZoomClamp(t *testing.T) {
	c := NewCamera(70)
	c.MinDist, c.MaxDist = 2, 50
	for i := 0; i < 100; i++ {
		c.ZoomBy(0.5)
	}
	if c.TargetDist < c.MinDist-1e-9 {
		t.Errorf("dist %g below min %g", c.TargetDist, c.MinDist)
	}
	for i := 0; i < 100; i++ {
		c.ZoomBy(2)
	}
	if c.TargetDist > c.MaxDist+1e-9 {
		t.Errorf("dist %g above max %g", c.TargetDist, c.MaxDist)
	}
}

func TestEaseConverges(t *testing.T) {
	c := NewCamera(70)
	c.TargetYaw = c.Yaw + 1
	c.TargetDist = c.Dist + 10
	moving := true
	for i := 0; i < 1000 && moving; i++ {
		moving = c.EaseStep()
	}
	if moving {
		t.Error("camera never settled")
	}
	if math.Abs(c.Yaw-c.TargetYaw) > 1e-3 || math.Abs(c.Dist-c.TargetDist) > 1e-3 {
		t.Errorf("did not reach target: yaw %g/%g dist %g/%g", c.Yaw, c.TargetYaw, c.Dist, c.TargetDist)
	}
}

func TestFrameFits(t *testing.T) {
	c := NewCamera(70)
	c.Frame(Vec3{}, 10, 100, 40)
	c.Yaw, c.Pitch = 0, 0
	c.Dist = c.TargetDist
	// A point on the bounding sphere edge should project inside the buffer.
	x, y, _, ok := c.Project(Vec3{0, 10, 0}, 100, 40)
	if !ok {
		t.Fatal("edge point culled")
	}
	if y < 0 || y >= 40 || x < 0 || x >= 100 {
		t.Errorf("edge point (%d,%d) outside 100x40 viewport", x, y)
	}
}
