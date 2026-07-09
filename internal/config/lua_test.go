package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mccullahz/nepenthe-cli/internal/keymap"
)

// writeLua writes content to path, creating parent directories as
// needed, and fails the test on error.
func writeLua(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// setupEnv points XDG_CONFIG_HOME and HOME at fresh, isolated
// directories under t.TempDir() so tests never touch the real user
// config, and returns (configHome, home).
func setupEnv(t *testing.T) (configHome, home string) {
	t.Helper()
	root := t.TempDir()
	configHome = filepath.Join(root, "xdg-config")
	home = filepath.Join(root, "home")
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", home)
	return configHome, home
}

func userInitPath(configHome string) string {
	return filepath.Join(configHome, "nepenthe", "init.lua")
}

func vaultInitPath(vaultDir string) string {
	return filepath.Join(vaultDir, ".nepenthe", "init.lua")
}

func TestLuaOptStyleAssignment(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.vault_dir = "/kb/vault"
nepenthe.opt.editor = "nvim"
`)

	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.VaultDir != "/kb/vault" {
		t.Errorf("VaultDir = %q, want /kb/vault", cfg.VaultDir)
	}
	if cfg.Editor != "nvim" {
		t.Errorf("Editor = %q, want nvim", cfg.Editor)
	}
}

func TestLuaSetupStyle(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.setup({
	vault_dir = "/kb/vault2",
	editor = "kak",
	theme = { style = "dracula", accent = "#ff0000", dim = "8" },
	graph = { link_distance = 5, repulsion = 10, iterations = 500, show_labels = false, fov = 90 },
})
`)

	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.VaultDir != "/kb/vault2" {
		t.Errorf("VaultDir = %q, want /kb/vault2", cfg.VaultDir)
	}
	if cfg.Editor != "kak" {
		t.Errorf("Editor = %q, want kak", cfg.Editor)
	}
	if cfg.Theme.GlamourStyle != "dracula" {
		t.Errorf("Theme.GlamourStyle = %q, want dracula", cfg.Theme.GlamourStyle)
	}
	if cfg.Theme.Accent != "#ff0000" {
		t.Errorf("Theme.Accent = %q, want #ff0000", cfg.Theme.Accent)
	}
	if cfg.Theme.Dim != "8" {
		t.Errorf("Theme.Dim = %q, want 8", cfg.Theme.Dim)
	}
	if cfg.Graph.LinkDistance != 5 {
		t.Errorf("Graph.LinkDistance = %v, want 5", cfg.Graph.LinkDistance)
	}
	if cfg.Graph.Repulsion != 10 {
		t.Errorf("Graph.Repulsion = %v, want 10", cfg.Graph.Repulsion)
	}
	if cfg.Graph.Iterations != 500 {
		t.Errorf("Graph.Iterations = %v, want 500", cfg.Graph.Iterations)
	}
	if cfg.Graph.ShowLabels != false {
		t.Errorf("Graph.ShowLabels = %v, want false", cfg.Graph.ShowLabels)
	}
	if cfg.Graph.FOV != 90 {
		t.Errorf("Graph.FOV = %v, want 90", cfg.Graph.FOV)
	}
}

func TestLuaThemeGlamourStyleAlias(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.setup({ theme = { glamour_style = "light" } })
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Theme.GlamourStyle != "light" {
		t.Errorf("Theme.GlamourStyle = %q, want light", cfg.Theme.GlamourStyle)
	}
}

func TestLuaTildeExpansion(t *testing.T) {
	configHome, home := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.vault_dir = "~/kb"
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	want := filepath.Join(home, "kb")
	if cfg.VaultDir != want {
		t.Errorf("VaultDir = %q, want %q", cfg.VaultDir, want)
	}
}

func TestLuaTildeAloneExpandsToHome(t *testing.T) {
	configHome, home := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.vault_dir = "~"
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.VaultDir != home {
		t.Errorf("VaultDir = %q, want %q", cfg.VaultDir, home)
	}
}

func TestLuaKeymapSetStringAndList(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.keymap.set("quit", "x")
nepenthe.keymap.set("help", {"?", "F1"})
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if got := cfg.Keymap[keymap.Quit]; len(got) != 1 || got[0] != "x" {
		t.Errorf("Keymap[Quit] = %v, want [x]", got)
	}
	if got := cfg.Keymap[keymap.Help]; len(got) != 2 || got[0] != "?" || got[1] != "F1" {
		t.Errorf("Keymap[Help] = %v, want [? F1]", got)
	}
}

func TestLuaKeymapInvalidActionCollectsErrorAndKeepsGoing(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.keymap.set("quit", "x")
nepenthe.keymap.set("bogus_action", "z")
nepenthe.opt.editor = "kakoune"
`)
	cfg := Default()
	err := loadLua(cfg, "")
	if err == nil {
		t.Fatal("expected error for unknown action, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_action") {
		t.Errorf("error %v does not mention bogus_action", err)
	}
	// Settings before and after the bad line should still have applied.
	if got := cfg.Keymap[keymap.Quit]; len(got) != 1 || got[0] != "x" {
		t.Errorf("Keymap[Quit] = %v, want [x]", got)
	}
	if cfg.Editor != "kakoune" {
		t.Errorf("Editor = %q, want kakoune", cfg.Editor)
	}
}

func TestLuaKeymapReset(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.keymap.set("quit", "x")
nepenthe.keymap.reset("quit")
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	want := keymap.Default()[keymap.Quit]
	got := cfg.Keymap[keymap.Quit]
	if len(got) != len(want) {
		t.Fatalf("Keymap[Quit] = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Keymap[Quit] = %v, want %v", got, want)
		}
	}
}

func TestLuaUnknownOptionCollectsError(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.not_a_real_option = "x"
nepenthe.opt.editor = "still-applied"
`)
	cfg := Default()
	err := loadLua(cfg, "")
	if err == nil {
		t.Fatal("expected error for unknown option, got nil")
	}
	if !strings.Contains(err.Error(), "not_a_real_option") {
		t.Errorf("error %v does not mention not_a_real_option", err)
	}
	if cfg.Editor != "still-applied" {
		t.Errorf("Editor = %q, want still-applied", cfg.Editor)
	}
}

func TestLuaWrongTypeCollectsError(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.vault_dir = 42
nepenthe.opt.editor = "still-applied"
`)
	cfg := Default()
	err := loadLua(cfg, "")
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
	if !strings.Contains(err.Error(), "vault_dir") {
		t.Errorf("error %v does not mention vault_dir", err)
	}
	if cfg.Editor != "still-applied" {
		t.Errorf("Editor = %q, want still-applied", cfg.Editor)
	}
}

func TestLuaVaultLocalOverridesUser(t *testing.T) {
	configHome, _ := setupEnv(t)
	vaultDir := t.TempDir()

	writeLua(t, userInitPath(configHome), `
nepenthe.opt.editor = "user-editor"
nepenthe.setup({ theme = { accent = "#111111" } })
`)
	writeLua(t, vaultInitPath(vaultDir), `
nepenthe.opt.editor = "vault-editor"
`)

	cfg := Default()
	if err := loadLua(cfg, vaultDir); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Editor != "vault-editor" {
		t.Errorf("Editor = %q, want vault-editor (vault-local should win)", cfg.Editor)
	}
	// Setting only present in the user config should survive.
	if cfg.Theme.Accent != "#111111" {
		t.Errorf("Theme.Accent = %q, want #111111 (from user config)", cfg.Theme.Accent)
	}
}

// nepenthe.vault names the vault this Load() call is for; it's the same
// value across both the user and vault-local passes (the vaultDir
// argument passed to loadLua), and empty when no vault is known yet.
func TestLuaVaultGlobalReflectsVaultDir(t *testing.T) {
	configHome, _ := setupEnv(t)
	vaultDir := t.TempDir()
	writeLua(t, userInitPath(configHome), `
if nepenthe.vault ~= "`+vaultDir+`" then error("expected vault dir on user pass, got " .. tostring(nepenthe.vault)) end
`)
	writeLua(t, vaultInitPath(vaultDir), `
if nepenthe.vault ~= "`+vaultDir+`" then error("expected vault dir, got " .. tostring(nepenthe.vault)) end
nepenthe.opt.editor = "marked-ok"
`)
	cfg := Default()
	if err := loadLua(cfg, vaultDir); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Editor != "marked-ok" {
		t.Errorf("Editor = %q, want marked-ok", cfg.Editor)
	}
}

func TestLuaVaultGlobalEmptyWhenNoVault(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
if nepenthe.vault ~= "" then error("expected empty vault, got " .. nepenthe.vault) end
nepenthe.opt.editor = "no-vault-ok"
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Editor != "no-vault-ok" {
		t.Errorf("Editor = %q, want no-vault-ok", cfg.Editor)
	}
}

func TestLuaVersionConstant(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
if nepenthe.version ~= "0.1.0" then error("unexpected version " .. tostring(nepenthe.version)) end
nepenthe.opt.editor = "version-ok"
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Editor != "version-ok" {
		t.Errorf("Editor = %q, want version-ok", cfg.Editor)
	}
}

func TestLuaMissingFilesAreNotAnError(t *testing.T) {
	configHome, _ := setupEnv(t)
	_ = configHome // no init.lua written anywhere
	vaultDir := filepath.Join(t.TempDir(), "does-not-exist")

	cfg := Default()
	want := Default()
	if err := loadLua(cfg, vaultDir); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.VaultDir != want.VaultDir || cfg.Editor != want.Editor || cfg.Theme != want.Theme || cfg.Graph != want.Graph {
		t.Errorf("cfg mutated despite missing files: got %+v, want %+v", cfg, want)
	}
}

func TestLuaSyntaxErrorInVaultLocalKeepsUserSettings(t *testing.T) {
	configHome, _ := setupEnv(t)
	vaultDir := t.TempDir()

	writeLua(t, userInitPath(configHome), `
nepenthe.opt.editor = "user-editor-survives"
`)
	// Deliberately malformed Lua (unterminated table constructor).
	writeLua(t, vaultInitPath(vaultDir), `
nepenthe.setup({ theme = { accent = "#000000"
`)

	cfg := Default()
	err := loadLua(cfg, vaultDir)
	if err == nil {
		t.Fatal("expected syntax error from vault-local config, got nil")
	}
	if cfg.Editor != "user-editor-survives" {
		t.Errorf("Editor = %q, want user-editor-survives despite vault-local syntax error", cfg.Editor)
	}
}

func TestLuaRuntimeErrorMidFileKeepsPriorSettings(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.editor = "applied-before-error"
error("boom")
nepenthe.opt.editor = "never-applied"
`)
	cfg := Default()
	err := loadLua(cfg, "")
	if err == nil {
		t.Fatal("expected runtime error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %v does not mention boom", err)
	}
	if cfg.Editor != "applied-before-error" {
		t.Errorf("Editor = %q, want applied-before-error", cfg.Editor)
	}
}

func TestLuaReadOptionsBack(t *testing.T) {
	configHome, _ := setupEnv(t)
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.vault_dir = "/foo/bar"
local d = nepenthe.opt.vault_dir
if d ~= "/foo/bar" then error("vault_dir readback mismatch: " .. tostring(d)) end

nepenthe.opt.editor = "ed1"
local e = nepenthe.opt.editor
nepenthe.opt.editor = e .. "-suffix"

nepenthe.setup({ theme = { accent = "#123456" } })
local t = nepenthe.opt.theme
if t.accent ~= "#123456" then error("theme readback mismatch: " .. tostring(t.accent)) end
`)
	cfg := Default()
	if err := loadLua(cfg, ""); err != nil {
		t.Fatalf("loadLua: %v", err)
	}
	if cfg.Editor != "ed1-suffix" {
		t.Errorf("Editor = %q, want ed1-suffix", cfg.Editor)
	}
	if cfg.Theme.Accent != "#123456" {
		t.Errorf("Theme.Accent = %q, want #123456", cfg.Theme.Accent)
	}
}

func TestLoadIntegration(t *testing.T) {
	configHome, _ := setupEnv(t)
	vaultDir := t.TempDir()
	writeLua(t, userInitPath(configHome), `
nepenthe.opt.editor = "from-load"
`)
	cfg, err := Load(vaultDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Editor != "from-load" {
		t.Errorf("Editor = %q, want from-load", cfg.Editor)
	}
}
