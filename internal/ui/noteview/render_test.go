package noteview

import (
	"strings"
	"testing"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

// TestRenderPipelineHighlights confirms the sentinel markers survive a
// real glamour render and are swapped out for styled text (no raw
// sentinels or brackets, alias text present).
func TestRenderPipelineHighlights(t *testing.T) {
	v := &vault.Vault{Notes: map[string]*vault.Note{
		"guide.md": {Path: "guide.md"},
	}}
	m := &Model{
		cfg:     config.Default(),
		v:       v,
		path:    "note.md",
		current: 0,
	}
	m.cfg.Theme.GlamourStyle = "dark"
	m.raw = "# Title\n\nSee [[guide|the guide]] for details.\n"
	m.buildStyles()
	m.links = extractLinks(m.raw)
	m.resolveLinks()
	m.SetSize(60, 10)

	out := m.applyHighlights(m.rendered)
	if strings.ContainsRune(out, sentOpen) || strings.ContainsRune(out, sentClose) {
		t.Errorf("sentinels leaked into rendered output")
	}
	if strings.Contains(out, "[[guide") {
		t.Errorf("raw wikilink brackets survived rendering")
	}
	if !strings.Contains(out, "the guide") {
		t.Errorf("alias display text missing from render:\n%s", out)
	}
}

func TestViewDimensions(t *testing.T) {
	v := &vault.Vault{Notes: map[string]*vault.Note{}}
	m := &Model{cfg: config.Default(), v: v, path: "note.md", current: -1}
	m.raw = strings.Repeat("line of text\n", 40)
	m.buildStyles()
	m.SetSize(40, 8)

	lines := strings.Split(m.View(), "\n")
	if len(lines) != 8 {
		t.Fatalf("View() produced %d lines, want 8", len(lines))
	}
}
