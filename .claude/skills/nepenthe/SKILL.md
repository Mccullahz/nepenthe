---
name: nepenthe
description: >-
  How to use nepenthe, the terminal knowledge base with a 3D link graph:
  create and organize notes and bases, navigate the graph and read mode,
  change settings, and script it with Lua (init.lua). Use when the user is
  creating, linking, or organizing notes in a nepenthe vault; navigating or
  configuring the app; or writing/debugging its init.lua.
---

# Using nepenthe

nepenthe is an Obsidian-style knowledge base for the terminal. A **vault**
is just a directory of Markdown files; notes are shown as a navigable **3D
link graph**, edited in your `$EDITOR`, and configured like neovim via Lua.

Launch it: `nepenthe [vault-dir]`. Vault resolution order:
positional arg / `-vault` flag → `$NEPENTHE_VAULT` → `vault_dir` in Lua
config → `~/nepenthe` (created on first run). Quit with `:q` from the graph
(`ctrl+c` is the emergency exit).

Canonical references in the repo: `README.md` (full docs) and
`examples/init.lua` (annotated config). Prefer those for exhaustive detail;
this skill is the working how-to.

---

## 1. Creating documents & knowledge bases

**A vault is plain files and folders.** There is no database or import step
— any directory of `.md` files is a valid vault, and you can edit it with
nepenthe or any other tool interchangeably.

**Create a note:** `:new <path>` (the `.md` is added if omitted). It creates
the file and opens it in read mode; press `i` or `a` to edit it in your
`$EDITOR`. Nested paths auto-create folders: `:new projects/web/api` makes
`projects/web/api.md`.

**Import / export:**
- `:import <src> [dest-dir]` — copy a file or directory of `.md` into the
  vault (optionally under `dest-dir`). Or just drop files into the folder
  and `:rescan`.
- `:export <note|.> <dest>` — copy a note (or `.` for the whole vault) out
  to `dest`.

**Link notes** — links become graph edges and are navigable in read mode:
- `[[wikilinks]]` — by note title or filename stem. Supports
  `[[target|alias]]` and `[[target#heading]]`. Resolution prefers an exact
  path, then a filename-stem match (shallowest path wins ties).
- `[text](relative/path.md)` — standard relative Markdown links.
- `http(s)://`, `mailto:`, and bare `#anchor` links are ignored (not edges).

**Frontmatter** (optional) at the very top of a file sets metadata and is
stripped from the rendered view:
```markdown
---
title: My Custom Title
tags: [zettel, draft]
---
# body starts here
```
Without a `title:`, the note's title is its first `# heading`, else its
filename.

**Bases = top-level folders.** The whole vault is one base by default
("all"). Each **top-level directory** is also its own base, named after the
folder, so grouping notes into folders gives you switchable sub-graphs. To
create a base, make a folder at the vault root and put notes in it. Rules:
- Only the **first path segment** names a base; deeper nesting does not add
  more bases (`projects/web/api.md` is in base `projects`).
- Notes at the vault root (no folder) are only in "all".
- A base's note count is recursive (everything beneath it).

To split one big vault into several: move notes into folders. To keep it as
one graph: leave them loose at the root.

---

## 2. Navigation

**The 3D graph (home screen).** Notes are nodes, links are lines; closer
nodes are larger/brighter with fuller titles.

| Key | Action |
|---|---|
| `h j k l` | orbit the camera |
| `K` / `J` (or `+` / `-`) | zoom in / out |
| `n` / `N` (or `tab` / `shift+tab`) | cycle selection (auto-centers it) |
| `enter` | open the selected note |
| `0` | reset the camera to frame the whole graph |
| `L` | toggle all titles on/off |
| `f` | toggle **focus mode** (spotlight the selected node + its direct links, dim the rest) |
| `b` | cycle the top-level bases |
| `/` | fuzzy search (see below) |

**Fuzzy "go to" search (`/`)** spans the **whole vault**, not just the
active base, and matches both notes (by title or path) and folders:
- Type to filter; **Tab / Shift+Tab** (or `↓`/`↑`) cycle results; **Enter**
  selects; **Esc** cancels.
- Selecting a **note** flies the camera to it, widening the base to "all"
  first if the note lives outside the current base.
- Selecting a **folder** scopes the graph to it (same as `:base <folder>`).

**Switching bases:**
- `b` cycles the top-level bases; the bottom bar shows the active one.
- `:base <name>` jumps directly. It accepts **any folder path at any
  depth** — `:base projects/web` scopes to that nested folder, not only the
  top-level bases `b` cycles.
- `:base` with no argument returns to the whole vault.
- Press **Tab** in the `:` line to complete folder/note paths (see §3).

**Read mode (viewing a note).** Markdown is rendered; links are navigable.

| Key | Action |
|---|---|
| `j k` / `ctrl+d ctrl+u` / `g G` | scroll / half-page / top-bottom |
| `tab` / `shift+tab` | cycle the links in the document |
| `enter` | follow the selected link |
| `esc` | step **back** through the link trail (never exits — no-op at the first note) |
| `i` or `a` | edit the note in `$EDITOR` (reloads on return) |
| `:q` | close the note (back to the note or graph beneath it) |

Read mode is vim-like: `esc` and `enter` only move between linked pages;
you leave a note with `:q`, and quit the app with `:q` from the graph.

**Global:** `?` help overlay · `:` command line · `ctrl+c` force-quit.

---

## 3. Settings

Two ways to configure: live from the running app, or in a Lua config file.

**Live, from the `:` command line:**
- `:set <option> <value>` — change a setting. Options:
  | Option | Value |
  |---|---|
  | `editor` | external editor command (else `$EDITOR`, then `vi`) |
  | `theme.style` | glamour style: `dark`, `light`, `dracula`, `auto`, or a path to a glamour JSON style |
  | `theme.accent` | highlight color: hex (`#7D56F4`) or ANSI index (`212`) |
  | `theme.dim` | de-emphasized/chrome color |
  | `graph.link_distance` | preferred edge length (number) |
  | `graph.repulsion` | node repulsion strength (number) |
  | `graph.fov` | camera field of view, degrees |
  | `graph.iterations` | layout iterations (integer) |
  | `graph.show_labels` | `true` / `false` |

  Theme changes apply to notes opened afterward; graph/layout numbers apply
  on the next rescan or base switch. **Focus mode** is not a `:set` option —
  toggle it live with `f`, or set its default in Lua (`graph.focus`).
- `:bind <action> <key> [key…]` — rebind an action live (see the action
  list in §4). Example: `:bind zoom_in K +`.

**Config files (loaded at startup, layered in this order):**
1. `~/.config/nepenthe/init.lua` (or `$XDG_CONFIG_HOME/nepenthe/init.lua`)
2. `<vault>/.nepenthe/init.lua` — vault-local overrides, applied on top.

A missing file is fine (defaults are used). See §4 for the Lua API.

---

## 4. Scripting with Lua (`init.lua`)

Config files are plain Lua (full standard library), each run in its own
fresh interpreter but applying to the same in-memory config. Runtime errors
(bad syntax, `error(...)`) abort the rest of that file but keep what already
applied; unknown option names or wrong-typed values are collected and
reported without aborting the script.

**Options — two equivalent styles.** Assign through `nepenthe.opt` (readable
and writable), or pass one `nepenthe.setup{}` table:

```lua
-- style (a): direct assignment
nepenthe.opt.vault_dir = "~/notes"   -- leading ~/ is expanded
nepenthe.opt.editor    = "nvim"      -- "" falls back to $EDITOR, then vi
nepenthe.opt.theme = {
  style  = "dracula",  -- alias: glamour_style
  accent = "#7D56F4",
  dim    = "240",
}
nepenthe.opt.graph = {
  link_distance = 3.0,
  repulsion     = 6.0,
  iterations    = 300,
  show_labels   = true,
  fov           = 70,
  focus         = true,   -- start in focus mode (toggle live with 'f')
}

-- style (b): one setup call (mix freely with the above)
nepenthe.setup({
  vault_dir = "~/notes",
  editor    = "nvim",
  theme = { style = "dracula", accent = "#ff79c6" },
  graph = { repulsion = 8.0, focus = true },
})

-- reads return the current value:
local v = nepenthe.opt.vault_dir
```

**Keymaps.** `nepenthe.keymap.set(action, keys)` replaces an action's
bindings; `keys` is a string or list of strings in bubbletea
`KeyMsg.String()` form (`"h"`, `"K"`, `"ctrl+d"`, `"shift+tab"`, `"enter"`,
`"esc"`, `"tab"`, `"left"`/`"right"`/`"up"`/`"down"`, `"pgup"`, `"F1"`, …).
`nepenthe.keymap.reset(action)` restores an action's default binding.

```lua
nepenthe.keymap.set("quit", "ctrl+q")
nepenthe.keymap.set("help", { "?", "F1" })
nepenthe.keymap.set("orbit_left", "left")   -- arrows only
-- nepenthe.keymap.reset("orbit_left")
```

**Rebindable actions:**
- Global: `quit`, `help`, `command`, `back`
  (`back` has no default key — quitting/closing is `:q`; bind it for a
  single-key close if you want one).
- Graph: `orbit_left`, `orbit_right`, `orbit_up`, `orbit_down`, `zoom_in`,
  `zoom_out`, `next_node`, `prev_node`, `open_node`, `reset_view`,
  `toggle_labels`, `toggle_focus`, `search`, `switch_base`.
- Read mode: `scroll_up`, `scroll_down`, `page_up`, `page_down`, `goto_top`,
  `goto_bottom`, `next_link`, `prev_link`, `follow_link`, `link_back`,
  `edit`.

**Introspection:**
- `nepenthe.version` — config API version string (e.g. for version-gating).
- `nepenthe.vault` — the vault directory this file is loaded for (`""` if
  none resolved yet). Useful for branching in a shared user config:
  ```lua
  if nepenthe.vault:match("work$") then
    nepenthe.opt.theme = { style = "light" }
  end
  ```

There is **no plugin/command API** yet — Lua configures options and
keymaps only.

---

## Quick task recipes

- **Start a new knowledge base:** `mkdir ~/notes && nepenthe ~/notes`, then
  `:new index` and press `i` to write it. Set `NEPENTHE_VAULT=~/notes` (or
  `nepenthe.opt.vault_dir`) so bare `nepenthe` always opens it.
- **Group notes into a base:** make a top-level folder (`:new area/foo`
  creates `area/`), then `b` / `:base area` to scope to it.
- **Find a note fast from anywhere:** `/`, type part of the title, Enter —
  it flies there across any base.
- **Jump into a deep folder:** `:base a/b/c` (Tab-completes), or `/` and
  pick the folder from the results.
- **Change the look now:** `:set theme.style dracula`, `:set theme.accent
  #ff79c6`; make it permanent in `~/.config/nepenthe/init.lua`.
