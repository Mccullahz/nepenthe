-- nepenthe init.lua — reference configuration
--
-- This file is documentation, not something nepenthe loads on its own.
-- Copy the parts you want into one of the real config locations:
--
--   ~/.config/nepenthe/init.lua              (user-wide config)
--   $XDG_CONFIG_HOME/nepenthe/init.lua        (if XDG_CONFIG_HOME is set,
--                                              it takes precedence over
--                                              ~/.config)
--   <vault>/.nepenthe/init.lua                (vault-local overrides,
--                                              applied on top of the
--                                              user config for that vault)
--
-- Both files run as plain Lua (the full standard library is available),
-- each in its own fresh interpreter, but they apply to the same
-- in-memory config — so the vault-local file only needs to set what it
-- wants to override.
--
-- A missing file is fine; nepenthe just falls back to defaults. Runtime
-- errors in a file (bad syntax, calling `error(...)`, etc.) abort the
-- rest of that file but keep whatever already applied. Bad option names
-- or wrong-typed values do NOT abort the script — they're collected and
-- reported together at the end, so the rest of the file still runs.

--------------------------------------------------------------------------
-- Options, style (a): direct assignment through nepenthe.opt
--------------------------------------------------------------------------
-- Every supported option can be read or written this way. Reads return
-- the option's current value (useful if you want to tweak something
-- relative to its default rather than replace it outright).

-- Where your notes live. A leading "~/" is expanded to your home
-- directory; other paths are used as-is.
nepenthe.opt.vault_dir = "~/notes"

-- External editor command for the "edit" action (i / a in read mode).
-- Leave unset (or "") to fall back to $EDITOR, then "vi".
nepenthe.opt.editor = "nvim"

-- theme and graph are tables; assign the whole table at once.
nepenthe.opt.theme = {
	style = "dracula", -- glamour style name ("dark", "light", "dracula",
	-- "auto", ...) or an absolute path to a glamour
	-- JSON style file. "glamour_style" is accepted
	-- as an alias for this same field.
	accent = "#7D56F4", -- lipgloss color for highlights: hex ("#7D56F4")
	-- or an ANSI index as a string ("212").
	dim = "240", -- lipgloss color for de-emphasized chrome.
}

nepenthe.opt.graph = {
	link_distance = 3.0, -- preferred edge length in layout space
	repulsion = 6.0, -- node-node repulsion strength
	iterations = 300, -- layout iterations before settling
	show_labels = true, -- draw titles next to nearby nodes
	fov = 70, -- camera field of view, in degrees
	focus = true, -- start in focus mode: dim everything except the
	-- selected node and its direct links (toggle with 'f')
	cluster = true, -- group notes by base into separated regions of the
	-- 3D space (each base its own constellation)
}

-- Read a value back:
-- local current_vault = nepenthe.opt.vault_dir

--------------------------------------------------------------------------
-- Options, style (b): a single nepenthe.setup({ ... }) call
--------------------------------------------------------------------------
-- Equivalent to the assignments above — pick whichever style you like,
-- or mix them. setup() is handy when you want everything in one block.

nepenthe.setup({
	vault_dir = "~/notes",
	editor = "nvim",
	theme = {
		style = "dracula",
		accent = "#7D56F4",
		dim = "240",
	},
	graph = {
		link_distance = 3.0,
		repulsion = 6.0,
		iterations = 300,
		show_labels = true,
		fov = 70,
		focus = true,
		cluster = true,
	},
})

--------------------------------------------------------------------------
-- Keymaps
--------------------------------------------------------------------------
-- nepenthe.keymap.set(action, keys) replaces the bindings for `action`.
-- `action` must be one of the built-in action names (see the full list
-- below); unknown action names are reported as errors but don't stop
-- the rest of the file from applying. `keys` is either a single key
-- string or a list of key strings, using bubbletea's KeyMsg.String()
-- form: plain runes ("h", "?"), "ctrl+x", "shift+tab", "enter", "esc",
-- "pgup", "pgdown", "tab", arrow names ("left", "right", "up", "down"),
-- function keys ("F1"), etc.

-- Single key:
nepenthe.keymap.set("quit", "ctrl+q")

-- Multiple keys bound to the same action:
nepenthe.keymap.set("help", { "?", "F1" })

-- Rebind graph orbiting to arrow keys only (drop h/j/k/l):
nepenthe.keymap.set("orbit_left", "left")
nepenthe.keymap.set("orbit_right", "right")
nepenthe.keymap.set("orbit_up", "up")
nepenthe.keymap.set("orbit_down", "down")

-- nepenthe.keymap.reset(action) restores a single action's stock
-- (vim-flavored) binding, undoing any nepenthe.keymap.set() for it.
-- nepenthe.keymap.reset("orbit_left")

-- Every rebindable action, for reference:
--
--   Global:          quit, help, back, command
--   Graph:            orbit_left, orbit_right, orbit_up, orbit_down,
--                     zoom_in, zoom_out, next_node, prev_node,
--                     open_node, reset_view, toggle_labels, toggle_focus,
--                     search, switch_base
--   Note viewing:     scroll_up, scroll_down, page_up, page_down,
--                     goto_top, goto_bottom, next_link, prev_link,
--                     follow_link, link_back, edit
--
-- "edit" opens the note in your external $EDITOR (there is no built-in
-- editor). "link_back" (esc) steps back through followed links without
-- ever leaving read mode; use :q to close a note.

--------------------------------------------------------------------------
-- Misc
--------------------------------------------------------------------------
-- nepenthe.version is the config API version string, e.g. for
-- version-gating parts of your config:
--
--   if nepenthe.version ~= "0.1.0" then
--     -- adjust for a different nepenthe version
--   end
--
-- nepenthe.vault is the vault directory this file is being loaded for
-- (matches the vault_dir resolved before Lua ran, or "" if none is
-- known yet). Handy in a vault-local .nepenthe/init.lua that wants to
-- confirm which vault it's customizing, or in a shared user config that
-- wants to branch on it:
--
--   if nepenthe.vault:match("work$") then
--     nepenthe.opt.theme = { style = "light" }
--   end
