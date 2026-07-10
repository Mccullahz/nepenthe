// Package config holds nepenthe's runtime configuration: defaults,
// overridden first by the user's Lua config (~/.config/nepenthe/init.lua),
// then by a vault-local .nepenthe/init.lua if present.
package config

import (
	"os"
	"path/filepath"

	"github.com/mccullahz/nepenthe-cli/internal/keymap"
)

// Theme controls colors and markdown rendering style.
type Theme struct {
	// GlamourStyle is a glamour style name ("dark", "light", "dracula",
	// "auto", ...) or an absolute path to a glamour JSON style file.
	GlamourStyle string
	Accent       string // lipgloss color for highlights, e.g. "#7D56F4" or "212"
	Dim          string // lipgloss color for de-emphasized chrome
}

// GraphConfig tunes the 3D graph view.
type GraphConfig struct {
	LinkDistance float64 // preferred edge length in layout space
	Repulsion    float64 // node-node repulsion strength
	Iterations   int     // layout iterations before settling
	ShowLabels   bool    // draw titles next to near nodes
	FOV          float64 // camera field of view in degrees
	// Focus dims everything except the selected node and its direct
	// neighbors, and hides non-incident edges, to cut the "hairball".
	Focus bool
	// Cluster groups notes by base into separated regions of the 3D space
	// so each knowledge base reads as its own constellation.
	Cluster bool
}

// Config is the full runtime configuration.
type Config struct {
	// VaultDir is the knowledge base root. Resolution order:
	// CLI argument > Lua config > $NEPENTHE_VAULT > ~/nepenthe.
	VaultDir string
	// Editor is the external editor command; empty means $EDITOR.
	Editor string
	Theme  Theme
	Graph  GraphConfig
	Keymap keymap.Keymap
}

// Default returns the stock configuration.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		VaultDir: filepath.Join(home, "nepenthe"),
		Editor:   "",
		Theme: Theme{
			GlamourStyle: "auto",
			Accent:       "#7D56F4",
			Dim:          "240",
		},
		Graph: GraphConfig{
			LinkDistance: 3.0,
			Repulsion:    6.0,
			Iterations:   300,
			ShowLabels:   true,
			FOV:          70,
			Focus:        true,
			Cluster:      true,
		},
		Keymap: keymap.Default(),
	}
}

// EditorCommand resolves the external editor to invoke.
func (c *Config) EditorCommand() string {
	if c.Editor != "" {
		return c.Editor
	}
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	return "vi"
}

// Load builds the effective config: defaults overlaid with the user's
// Lua config, then with vaultDir/.nepenthe/init.lua when vaultDir is
// known ("" skips the vault-local pass). Lua errors are returned but the
// config is still usable (defaults plus whatever applied cleanly).
func Load(vaultDir string) (*Config, error) {
	cfg := Default()
	err := loadLua(cfg, vaultDir)
	return cfg, err
}
