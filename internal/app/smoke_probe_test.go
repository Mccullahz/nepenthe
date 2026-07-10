package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/ui"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

// drain runs a command tree to completion, feeding every produced
// message back into the model, like the bubbletea runtime would.
func drain(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		// Skip animation ticks so this terminates.
		if fmt.Sprintf("%T", msg) == "ui.GraphTickMsg" {
			continue
		}
		var next tea.Cmd
		m, next = m.Update(msg)
		queue = append(queue, next)
	}
	return m
}

func TestEnterOpensNote(t *testing.T) {
	v, err := vault.Open("../../examples/vault")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	var m tea.Model = New(cfg, v, "")

	m = drain(t, m, func() tea.Msg { return tea.WindowSizeMsg{Width: 140, Height: 40} })

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = drain(t, m, cmd)

	a := m.(*App)
	t.Logf("stack depth: %d, mode: %s", len(a.stack), a.modeName())
	if a.modeName() != "note" {
		t.Fatalf("expected note view on top after enter, got %q (stack %d)", a.modeName(), len(a.stack))
	}
}

// TestGraphTickReachesGraphWhileNoteOpen guards the animation-deadlock fix:
// a graph tick that fires while a note is on top must still be delivered to
// the graph (stack[0]) so its loop keeps running, not eaten by the note.
func TestGraphTickReachesGraphWhileNoteOpen(t *testing.T) {
	v, err := vault.Open("../../examples/vault")
	if err != nil {
		t.Fatal(err)
	}
	var m tea.Model = New(config.Default(), v, "")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})

	// Kick the camera so the graph's animation loop is active/ticking.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	// Open a note: the graph is no longer the top view.
	m, _ = m.Update(ui.OpenNoteMsg{Path: "index.md"})
	if a := m.(*App); a.modeName() != "note" {
		t.Fatalf("expected a note on top, got %q", a.modeName())
	}

	// The tick must route to the graph and produce the next tick (loop alive).
	m, cmd := m.Update(ui.GraphTickMsg{})
	if cmd == nil {
		t.Fatal("graph tick under a note produced no follow-up: the animation loop would deadlock")
	}
	if _, ok := cmd().(ui.GraphTickMsg); !ok {
		t.Fatalf("follow-up = %T, want ui.GraphTickMsg (graph should reschedule)", cmd())
	}
	if a := m.(*App); a.modeName() != "note" {
		t.Errorf("note should still be on top after the tick, got %q", a.modeName())
	}
}
