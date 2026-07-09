// nepenthe — a knowledge base that lives in your terminal.
//
//	nepenthe [flags] [vault-dir]
//
// Vault resolution order: positional argument / -vault flag,
// $NEPENTHE_VAULT, the Lua config's vault_dir, then ~/nepenthe.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/mccullahz/nepenthe-cli/internal/app"
	"github.com/mccullahz/nepenthe-cli/internal/config"
	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

const welcome = `# Welcome to nepenthe

Your vault was just created. A few things to try:

- Press ` + "`?`" + ` for keybindings.
- Press ` + "`:`" + ` and type ` + "`new ideas/first-note`" + ` to create a note.
- Link notes with ` + "`[[wikilinks]]`" + ` — they become edges in the graph.
- Drop existing markdown files anywhere in this directory, or use
  ` + "`:import <path>`" + `.

See [[guide]] for more.
`

const guide = `# Guide

nepenthe treats this directory as one big knowledge base. Every
top-level folder is also addressable as its own smaller base — press
` + "`b`" + ` in the graph to cycle bases, or ` + "`:base <name>`" + `.

Configuration lives in ` + "`~/.config/nepenthe/init.lua`" + ` and can also be
placed per-vault in ` + "`.nepenthe/init.lua`" + `. Back to [[welcome]].
`

func run() error {
	var vaultFlag string
	flag.StringVar(&vaultFlag, "vault", "", "path to the vault (knowledge base root)")
	flag.Parse()

	dir := vaultFlag
	if dir == "" && flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if dir == "" {
		dir = os.Getenv("NEPENTHE_VAULT")
	}

	// First pass learns vault_dir from the user config; second pass also
	// applies the vault-local config once the vault is known.
	cfg, cfgErr := config.Load("")
	if dir == "" {
		dir = cfg.VaultDir
	}
	cfg, cfgErr = config.Load(dir)
	cfg.VaultDir = dir

	// Resolve "auto" markdown styling once, up front. Detecting the
	// terminal background later would race Bubble Tea's input reader
	// (the OSC query response is read from stdin) and can eat keys.
	if s := cfg.Theme.GlamourStyle; s == "" || strings.EqualFold(s, "auto") {
		if termenv.HasDarkBackground() {
			cfg.Theme.GlamourStyle = "dark"
		} else {
			cfg.Theme.GlamourStyle = "light"
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating vault %s: %w", dir, err)
	}

	v, err := vault.Open(dir)
	if err != nil {
		return fmt.Errorf("opening vault %s: %w", dir, err)
	}
	if len(v.Notes) == 0 {
		if err := v.Write("welcome.md", welcome); err != nil {
			return err
		}
		if err := v.Write("guide.md", guide); err != nil {
			return err
		}
		if err := v.Rescan(); err != nil {
			return err
		}
	}

	status := ""
	if cfgErr != nil {
		status = "config: " + cfgErr.Error()
	}

	p := tea.NewProgram(app.New(cfg, v, status), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "nepenthe:", err)
		os.Exit(1)
	}
}
