package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mccullahz/nepenthe-cli/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

// luaConfigVersion is exposed to scripts as nepenthe.version.
const luaConfigVersion = "0.1.0"

// loadLua applies ~/.config/nepenthe/init.lua (or
// $XDG_CONFIG_HOME/nepenthe/init.lua when set) and, if vaultDir is
// non-empty, vaultDir/.nepenthe/init.lua on top of cfg. Missing files are
// not an error. Each file runs in its own fresh Lua state; errors from
// both passes are joined and returned, but every setting that applied
// cleanly (from either file, up to the point of any error) remains on
// cfg.
func loadLua(cfg *Config, vaultDir string) error {
	var errs []error

	if err := runLuaFile(cfg, userConfigPath(), vaultDir); err != nil {
		errs = append(errs, err)
	}

	if vaultDir != "" {
		vaultPath := filepath.Join(vaultDir, ".nepenthe", "init.lua")
		if err := runLuaFile(cfg, vaultPath, vaultDir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// userConfigPath resolves the user-level init.lua location.
func userConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nepenthe", "init.lua")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nepenthe", "init.lua")
}

// runLuaFile executes a single init.lua against cfg in a fresh Lua
// state. A missing file is not an error. Config-level problems (unknown
// options, wrong types, unknown keymap actions) are collected and
// returned as a single joined error after the whole script has run;
// genuine Lua runtime/syntax errors abort the rest of that file but
// leave whatever already applied in place.
func runLuaFile(cfg *Config, path, vaultDir string) error {
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("%s: %w", path, statErr)
	}

	L := lua.NewState()
	defer L.Close()

	env := &luaEnv{cfg: cfg, path: path}
	env.install(L, vaultDir)

	var errs []error
	if err := L.DoFile(path); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", path, err))
	}
	errs = append(errs, env.errs...)

	return errors.Join(errs...)
}

// luaEnv holds the state needed while wiring the `nepenthe` global into
// a Lua VM and applying whatever it does to cfg.
type luaEnv struct {
	cfg  *Config
	path string // source file, for error messages
	errs []error
}

// errorf builds a descriptive "file: key: message" error and does not
// itself abort script execution; callers append it to env.errs.
func (env *luaEnv) errorf(key, format string, args ...any) error {
	return fmt.Errorf("%s: %s: %s", env.path, key, fmt.Sprintf(format, args...))
}

// install wires the `nepenthe` global table into L.
func (env *luaEnv) install(L *lua.LState, vaultDir string) {
	nepenthe := L.NewTable()
	nepenthe.RawSetString("version", lua.LString(luaConfigVersion))
	nepenthe.RawSetString("vault", lua.LString(vaultDir))

	nepenthe.RawSetString("opt", env.newOptTable(L))
	nepenthe.RawSetString("setup", L.NewFunction(env.luaSetup))
	nepenthe.RawSetString("keymap", env.newKeymapTable(L))

	L.SetGlobal("nepenthe", nepenthe)
}

// newOptTable builds nepenthe.opt: an empty table whose metatable routes
// reads/writes of arbitrary keys through env.getOption/env.setOption, so
// `nepenthe.opt.vault_dir = "x"` and `local v = nepenthe.opt.vault_dir`
// both work without pre-declaring every field.
func (env *luaEnv) newOptTable(L *lua.LState) *lua.LTable {
	opt := L.NewTable()
	mt := L.NewTable()

	mt.RawSetString("__newindex", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		val := L.CheckAny(3)
		if err := env.setOption(key, val); err != nil {
			env.errs = append(env.errs, err)
		}
		return 0
	}))
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		L.Push(env.getOption(L, key))
		return 1
	}))

	L.SetMetatable(opt, mt)
	return opt
}

// luaSetup implements nepenthe.setup({ ... }).
func (env *luaEnv) luaSetup(L *lua.LState) int {
	tbl := L.CheckTable(1)
	tbl.ForEach(func(k, v lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			env.errs = append(env.errs, env.errorf("setup", "option keys must be strings, got %s", k.Type()))
			return
		}
		if err := env.setOption(string(ks), v); err != nil {
			env.errs = append(env.errs, err)
		}
	})
	return 0
}

// setOption applies a single top-level option (used by both
// nepenthe.opt.<key> = v and nepenthe.setup({ <key> = v })).
func (env *luaEnv) setOption(key string, val lua.LValue) error {
	switch key {
	case "vault_dir":
		s, ok := val.(lua.LString)
		if !ok {
			return env.errorf(key, "expected string, got %s", val.Type())
		}
		env.cfg.VaultDir = expandHome(string(s))
	case "editor":
		s, ok := val.(lua.LString)
		if !ok {
			return env.errorf(key, "expected string, got %s", val.Type())
		}
		env.cfg.Editor = string(s)
	case "theme":
		tbl, ok := val.(*lua.LTable)
		if !ok {
			return env.errorf(key, "expected table, got %s", val.Type())
		}
		return env.applyTheme(tbl)
	case "graph":
		tbl, ok := val.(*lua.LTable)
		if !ok {
			return env.errorf(key, "expected table, got %s", val.Type())
		}
		return env.applyGraph(tbl)
	default:
		return env.errorf(key, "unknown option")
	}
	return nil
}

// getOption reads a single top-level option back, for `local v =
// nepenthe.opt.<key>`. theme/graph are returned as fresh snapshot
// tables. Unknown keys return nil rather than erroring, matching Lua's
// usual table-read semantics.
func (env *luaEnv) getOption(L *lua.LState, key string) lua.LValue {
	switch key {
	case "vault_dir":
		return lua.LString(env.cfg.VaultDir)
	case "editor":
		return lua.LString(env.cfg.Editor)
	case "theme":
		t := L.NewTable()
		t.RawSetString("style", lua.LString(env.cfg.Theme.GlamourStyle))
		t.RawSetString("glamour_style", lua.LString(env.cfg.Theme.GlamourStyle))
		t.RawSetString("accent", lua.LString(env.cfg.Theme.Accent))
		t.RawSetString("dim", lua.LString(env.cfg.Theme.Dim))
		return t
	case "graph":
		t := L.NewTable()
		t.RawSetString("link_distance", lua.LNumber(env.cfg.Graph.LinkDistance))
		t.RawSetString("repulsion", lua.LNumber(env.cfg.Graph.Repulsion))
		t.RawSetString("iterations", lua.LNumber(env.cfg.Graph.Iterations))
		t.RawSetString("show_labels", lua.LBool(env.cfg.Graph.ShowLabels))
		t.RawSetString("fov", lua.LNumber(env.cfg.Graph.FOV))
		t.RawSetString("focus", lua.LBool(env.cfg.Graph.Focus))
		t.RawSetString("cluster", lua.LBool(env.cfg.Graph.Cluster))
		return t
	default:
		return lua.LNil
	}
}

// applyTheme applies fields of a theme = { ... } table.
func (env *luaEnv) applyTheme(tbl *lua.LTable) error {
	var errs []error
	tbl.ForEach(func(k, v lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			errs = append(errs, env.errorf("theme", "keys must be strings, got %s", k.Type()))
			return
		}
		key := "theme." + string(ks)
		switch string(ks) {
		case "style", "glamour_style":
			s, ok := v.(lua.LString)
			if !ok {
				errs = append(errs, env.errorf(key, "expected string, got %s", v.Type()))
				return
			}
			env.cfg.Theme.GlamourStyle = string(s)
		case "accent":
			s, ok := v.(lua.LString)
			if !ok {
				errs = append(errs, env.errorf(key, "expected string, got %s", v.Type()))
				return
			}
			env.cfg.Theme.Accent = string(s)
		case "dim":
			s, ok := v.(lua.LString)
			if !ok {
				errs = append(errs, env.errorf(key, "expected string, got %s", v.Type()))
				return
			}
			env.cfg.Theme.Dim = string(s)
		default:
			errs = append(errs, env.errorf(key, "unknown option"))
		}
	})
	return errors.Join(errs...)
}

// applyGraph applies fields of a graph = { ... } table.
func (env *luaEnv) applyGraph(tbl *lua.LTable) error {
	var errs []error
	tbl.ForEach(func(k, v lua.LValue) {
		ks, ok := k.(lua.LString)
		if !ok {
			errs = append(errs, env.errorf("graph", "keys must be strings, got %s", k.Type()))
			return
		}
		key := "graph." + string(ks)
		switch string(ks) {
		case "link_distance":
			n, ok := v.(lua.LNumber)
			if !ok {
				errs = append(errs, env.errorf(key, "expected number, got %s", v.Type()))
				return
			}
			env.cfg.Graph.LinkDistance = float64(n)
		case "repulsion":
			n, ok := v.(lua.LNumber)
			if !ok {
				errs = append(errs, env.errorf(key, "expected number, got %s", v.Type()))
				return
			}
			env.cfg.Graph.Repulsion = float64(n)
		case "iterations":
			n, ok := v.(lua.LNumber)
			if !ok {
				errs = append(errs, env.errorf(key, "expected number, got %s", v.Type()))
				return
			}
			env.cfg.Graph.Iterations = int(n)
		case "show_labels":
			b, ok := v.(lua.LBool)
			if !ok {
				errs = append(errs, env.errorf(key, "expected boolean, got %s", v.Type()))
				return
			}
			env.cfg.Graph.ShowLabels = bool(b)
		case "focus":
			b, ok := v.(lua.LBool)
			if !ok {
				errs = append(errs, env.errorf(key, "expected boolean, got %s", v.Type()))
				return
			}
			env.cfg.Graph.Focus = bool(b)
		case "cluster":
			b, ok := v.(lua.LBool)
			if !ok {
				errs = append(errs, env.errorf(key, "expected boolean, got %s", v.Type()))
				return
			}
			env.cfg.Graph.Cluster = bool(b)
		case "fov":
			n, ok := v.(lua.LNumber)
			if !ok {
				errs = append(errs, env.errorf(key, "expected number, got %s", v.Type()))
				return
			}
			env.cfg.Graph.FOV = float64(n)
		default:
			errs = append(errs, env.errorf(key, "unknown option"))
		}
	})
	return errors.Join(errs...)
}

// newKeymapTable builds nepenthe.keymap with set() and reset().
func (env *luaEnv) newKeymapTable(L *lua.LState) *lua.LTable {
	km := L.NewTable()
	km.RawSetString("set", L.NewFunction(env.luaKeymapSet))
	km.RawSetString("reset", L.NewFunction(env.luaKeymapReset))
	return km
}

// luaKeymapSet implements nepenthe.keymap.set(action, keys), where keys
// is a single key string or a list of key strings.
func (env *luaEnv) luaKeymapSet(L *lua.LState) int {
	actionStr := L.CheckString(1)
	arg := L.CheckAny(2)

	var keys []string
	switch v := arg.(type) {
	case lua.LString:
		keys = []string{string(v)}
	case *lua.LTable:
		v.ForEach(func(_, item lua.LValue) {
			s, ok := item.(lua.LString)
			if !ok {
				env.errs = append(env.errs, env.errorf("keymap.set", "keys list must contain only strings, got %s", item.Type()))
				return
			}
			keys = append(keys, string(s))
		})
	default:
		env.errs = append(env.errs, env.errorf("keymap.set", "keys must be a string or a list of strings, got %s", arg.Type()))
		return 0
	}

	action, ok := validAction(actionStr)
	if !ok {
		env.errs = append(env.errs, env.errorf("keymap.set", "unknown action %q", actionStr))
		return 0
	}
	env.cfg.Keymap.Set(action, keys...)
	return 0
}

// luaKeymapReset implements nepenthe.keymap.reset(action), restoring
// that action's stock binding.
func (env *luaEnv) luaKeymapReset(L *lua.LState) int {
	actionStr := L.CheckString(1)
	action, ok := validAction(actionStr)
	if !ok {
		env.errs = append(env.errs, env.errorf("keymap.reset", "unknown action %q", actionStr))
		return 0
	}
	def := keymap.Default()
	env.cfg.Keymap.Set(action, def[action]...)
	return 0
}

// validAction reports whether s names a real, rebindable action.
func validAction(s string) (keymap.Action, bool) {
	a := keymap.Action(s)
	for _, valid := range keymap.Actions() {
		if valid == a {
			return a, true
		}
	}
	return "", false
}

// expandHome expands a leading "~" or "~/" in p to the user's home
// directory. Paths that don't start with ~ are returned unchanged.
func expandHome(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
