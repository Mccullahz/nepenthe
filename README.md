# nepenthe

A knowledge base that lives in your terminal — an Obsidian-style tool
built on [charmbracelet](https://github.com/charmbracelet)'s stack, with
your notes rendered as a navigable **3D link graph** inside the CLI.

- **3D graph home screen** — notes are nodes floating in space, links are
  drawn as continuous lines. Orbit, zoom, and fly through your vault with
  vim motions; depth is conveyed with glyph size, brightness, and titles
  that grow as nodes come closer (toggle titles with `L`). Press `/` to
  fuzzy-search **notes and folders across every base** — pick a note and
  the camera flies to it (widening the base if it lives elsewhere), pick a
  folder to scope the graph to it. **Focus mode** (`f`)
  spotlights the selected node and its direct links — accented nodes and
  edges over a dimmed-but-visible rest — so the neighborhood stands out
  without hiding the surrounding structure.
- **Markdown, viewable and edited in `$EDITOR`** — read mode renders
  documents with glamour (Confluence/Obsidian-style); press `i`/`a` to
  open the note in your own `$EDITOR`, and it reloads when you're done.
  Read mode is vim-like: `esc` walks back through the links you followed,
  `:q` closes the note.
- **Plain files, plain folders** — a vault is just a directory of `.md`
  files. Drop files in, or `:import` / `:export` from the UI. `[[wikilinks]]`
  and relative markdown links become graph edges.
- **One big base, or many** — the whole vault is one knowledge base by
  default; every top-level folder is also addressable as its own base
  (`b` to cycle, `:base <name>`). In the 3D view notes **cluster by base**
  into separated regions, so each base reads as its own constellation
  (turn it off with `graph.cluster = false`).
- **Configured like neovim** — `~/.config/nepenthe/init.lua` (plus an
  optional per-vault `.nepenthe/init.lua`), or live from the UI with
  `:set` and `:bind`.

## Install

Requires Go 1.26+ and a truecolor-capable terminal (iTerm2, Ghostty,
WezTerm, Kitty, Alacritty; macOS Terminal.app only does 256 colors).

**Try it without installing:**

```sh
make run                    # builds ./nepenthe and opens the demo vault
# or: go build -o nepenthe . && ./nepenthe examples/vault
```

**Install it as a command** so `nepenthe` runs from anywhere:

```sh
make install                # builds and copies to ~/.local/bin/nepenthe
```

If `~/.local/bin` isn't on your `PATH`, add it (zsh is the macOS default):

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

Prefer a system location? `make install PREFIX=/usr/local` (may need
`sudo`). Remove it with `make uninstall`. Cross-compile release binaries
for macOS/Linux (arm64 + amd64) into `./dist` with `make dist`.

> Note: `go install .` also works but names the binary `nepenthe-cli`
> (after the module path), so use `make install` / `go build -o nepenthe`
> to get a plain `nepenthe` command.

**Point it at your knowledge base** so the bare command is
cwd-independent — otherwise `nepenthe` with no argument opens `~/nepenthe`:

```sh
export NEPENTHE_VAULT="$HOME/notes"          # in your shell rc, or…
# …set it in ~/.config/nepenthe/init.lua:  nepenthe.opt.vault_dir = "~/notes"
```

Now `nepenthe` opens your vault from anywhere, and `nepenthe ~/work/wiki`
opens a specific one when you want it.

## Usage

```sh
nepenthe [flags] [vault-dir]
```

Vault resolution order: positional arg / `-vault` flag → `$NEPENTHE_VAULT`
→ `vault_dir` from your Lua config → `~/nepenthe` (created on first run).

### Default keys (all rebindable)

| Context | Key | Action |
|---|---|---|
| anywhere | `?` | help overlay |
| anywhere | `:` | command line |
| anywhere | `ctrl+c` | force-quit (emergency) |
| graph | `:q` | quit nepenthe |
| graph | `h j k l` | orbit |
| graph | `K` / `J` (or `+`/`-`) | zoom in / out |
| graph | `n` / `N` (or tab) | cycle node selection (auto-centers) |
| graph | `/` | fuzzy-search notes + folders (all bases) |
| graph | `f` | toggle focus mode |
| graph | `enter` | open selected note |
| graph | `0` | reset camera |
| graph | `L` | toggle labels |
| graph | `b` | cycle knowledge base |
| note | `j k ctrl+d ctrl+u g G` | scroll |
| note | `tab` / `shift+tab` | cycle links |
| note | `enter` | follow selected link |
| note | `esc` | back through the link trail (never exits) |
| note | `i` or `a` | edit in `$EDITOR` |
| note | `:q` | close the note |

In the `/` search overlay: **Tab** / **Shift+Tab** (or `↓`/`↑`) cycle
results, **Enter** selects (fly to a note / scope to a folder), **Esc**
cancels.

### Commands

`:new <path>` · `:open <path>` · `:delete <path>` · `:base [name]` ·
`:import <src> [dest-dir]` · `:export <note|.> <dest>` · `:rescan` ·
`:set <option> <value>` · `:bind <action> <key> [key…]` · `:q`

`:q` closes the current note (returning to the note or graph beneath it);
on the graph it quits the app.

`:base` accepts **any folder path, at any depth** — `:base projects/web`
scopes to that nested folder, not just the top-level bases that `b`
cycles. Press **Tab** to complete the argument of `:base` (folders) and
`:open` / `:delete` (note paths); repeated Tab cycles the matches,
Shift+Tab reverses.

## Configuration

`~/.config/nepenthe/init.lua` (see [examples/init.lua](examples/init.lua)
for the full reference):

```lua
nepenthe.opt.vault_dir = "~/kb"
nepenthe.opt.editor = "nvim"

nepenthe.setup({
  theme = { style = "dracula", accent = "#ff79c6" },
  graph = { link_distance = 3.0, repulsion = 6.0, show_labels = true,
            focus = true },
})

nepenthe.keymap.set("zoom_in", { "K", "+" })
nepenthe.keymap.set("search", "/")
```

A vault-local `.nepenthe/init.lua` overrides the user config per vault.

## Development

```sh
go test ./...          # unit tests (vault, graph3d, views, lua config)
go build -o nepenthe .
```

End-to-end smoke tests drive the real TUI in a PTY (answering the
terminal's OSC color queries like a real emulator, sending keys one at
a time like a real human):

```sh
python3 scripts/ptydrive.py /tmp/out.log scripts/journey.keys -- ./nepenthe examples/vault
```

`scripts/journey.keys` walks the graph (search `/`, focus `f`, open a
note), exercises read-mode link navigation and the `esc`/`:q` semantics,
and quits; `scripts/commands.keys` exercises `:new`, `:base`, and base
cycling. To test the `$EDITOR` edit path non-interactively, point
`EDITOR` at a script (e.g. one that appends a line) before running.

## Architecture

```
main.go                 flags, vault resolution, first-run bootstrap
internal/app            root Bubble Tea model: view stack, routing,
                        ':' command line, status bar
internal/ui             view interface + message protocol (leaf package)
internal/ui/graphview   3D graph view (home screen): fly-to search,
                        focus mode, orbit/zoom, selection
internal/graph3d        force-directed 3D layout, orbit camera,
                        cell-buffer rasterizer
internal/ui/noteview    glamour-rendered read mode, link navigation
internal/vault          note index, wikilink resolution, graph,
                        bases, import/export
internal/config         defaults + Lua config (gopher-lua)
internal/keymap         rebindable action table
```

Views never import each other or the shell; they communicate by
emitting messages from `internal/ui`, which the shell routes.
