# Notebot

Local-first note capture with an on-device model and a web dashboard. Type a
thought from the terminal, have it titled and filed automatically, and browse
everything from a password-protected dashboard served on `127.0.0.1`. Nothing
leaves the machine.

Notebot is two binaries and an embedded web app:

- **`nb`** — capture and manage notes from the terminal.
- **`notebotd`** — a daemon that loads one [Needle3](https://cactuscompute.com/needle) model,
  exposes a REST API, and serves the dashboard.

## Features

- Capture with plain words — `nb today i wanted to write a poem`.
- Automatic titles and folder routing, with deterministic fallbacks.
- Full-text and date-range queries (`--since`, `--until`, `today`, `3 days ago`).
- Note and folder management: edit, move, delete, rename.
- Natural-language requests via `nb ask` and an in-dashboard agent panel, with
  confirmation for destructive actions.
- Markdown files on disk plus a SQLite index (FTS5), so notes stay readable and
  greppable outside the app.
- React + Tailwind dashboard embedded in the daemon binary.

## Requirements

- Linux `x86_64` (the vendored Needle3 engine targets this platform only).
- Go 1.22 or newer.
- Node.js 18+ and npm — only to build the dashboard.
- The Needle3 weights (`models/needle3.cact`) and engine library.

## Installation

```bash
make web                              # build the dashboard into web/dist
make worker && make nb && make daemon # build binaries into bin/
make install                          # install nb, notebotd, needle-worker to ~/.local/bin
```

Ensure `~/.local/bin` is on your `PATH`.

### Run as a service

```bash
nb install-service            # write, enable, and start a systemd user unit
nb install-service --no-start # write it without enabling
nb uninstall-service          # stop and remove it
```

`install-service` writes `~/.config/systemd/user/notebot.service` with the
absolute daemon path, your resolved data directory, and the configured port, then
runs `systemctl --user enable --now notebot`. This avoids the daemon resolving a
different data directory than the one `nb setup` used.

## Quick start

```bash
nb setup                              # copy the model, set the dashboard password
nb "I will go to Nilkhet to buy some books on 21 November"
nb today i wanted to write a poem
nb list

notebotd                              # serves the dashboard at http://127.0.0.1:8765
```

## Usage

### Capture

Quoted text and bare words both work. Bare words are captured when the first
argument is not a subcommand.

```bash
nb "deadline for the project is 20 October"
nb today i wanted to write a poem
nb "buy milk and eggs" --folder shopping   # force a folder
nb "draft the release notes" --dry-run     # preview without saving
```

If the text starts with a management verb (`update`, `change`, `delete`,
`remove`, `move`) **and** resolves to an existing note or folder, it is treated
as a request instead of a note:

```bash
nb "update the Chores note with: I will do my chores tomorrow"
nb "move the milk note to shopping"
nb "delete the old plan note"
```

Otherwise it is saved as a note — so `nb "Move the meeting to friday"` is
captured, not interpreted. Requests that change existing notes ask for
confirmation.

If a captured note's refined title matches an existing note, `nb` asks before
creating a second copy:

```
A note titled "Chores" already exists:
  448facd4  [inbox]  I will do my chores tomorrow

New text:
  I will do my chores today!

  [u]pdate the existing note, [n]ew note, [c]ancel?
```

When stdin is not a terminal, the note is created without prompting; pass
`--new` to skip the prompt explicitly.

### Query

| Command | Description |
| --- | --- |
| `nb list` / `nb ls` | All notes. |
| `nb list <folder>` | Notes in a folder. |
| `nb list --since yesterday --until now` | Time-bounded listing. |
| `nb today` / `nb yesterday` | Notes created today or yesterday. |
| `nb find <text>` | Full-text search. |
| `nb find <text> --since "1 week ago"` | Search within a time range. |
| `nb show <id>` | Print one note (ID prefixes are accepted). |

`--since` and `--until` accept `now`, `today`, `yesterday`, `3 days ago`,
`2 hours ago`, `2026-09-20`, `2026-09-01 14:30`, and similar. Filtering uses the
note's created time; pass `--updated` to filter on modification time instead.

### Manage

| Command | Description |
| --- | --- |
| `nb edit <id> --title "…" --folder work` | Update fields directly. |
| `nb edit <id>` | Open the note in `$EDITOR`. |
| `nb mv <id> <folder>` | Move a note. |
| `nb rm <id>` | Delete a note (prompts; `-y` to skip). |
| `nb mkfolder <name>` | Create a folder. |
| `nb rename <old> <new>` | Rename a folder. |
| `nb rmfolder <name>` | Delete a folder; its notes move to `inbox`. |
| `nb folders` | List folders. |

### Agent

`nb ask` accepts natural-language requests. Explicit verbs — move, delete, and
rename folder — are resolved deterministically; other requests are handled by
the model's tool calling.

```bash
nb ask "move the milk note to shopping"
nb ask "delete the notebot deadline note"
nb ask "rename the weekend folder to errands"
nb ask -y "create a note about the standup"
```

The dashboard exposes the same agent through a side panel. Destructive actions
require explicit confirmation in both interfaces.

## Configuration

### Environment variables

| Variable | Description |
| --- | --- |
| `NOTEBOT_DATA` | Data directory (default: `$XDG_DATA_HOME/notebot`, else `~/.notebot`). |
| `NOTEBOT_PORT` | Daemon port (default: `8765`). |
| `NEEDLE_MODEL` | Override the model weights path. |
| `NEEDLE_ENGINE` | Override the engine shared library path. |
| `NEEDLE_WORKER` | Override the `needle-worker` binary path. |

### `config.json`

Stored in the data directory (mode `0600`):

```json
{
  "port": 8765,
  "data_dir": "/home/user/.local/share/notebot",
  "model_path": "/home/user/.local/share/notebot/models/needle3.cact",
  "password_hash": "$2a$10$…",
  "session_secret": "…"
}
```

Read values with `nb config get <model_path|port|data_dir>`, and change them
with `nb config set <model_path|port> <value>`. Changing either requires a
daemon restart.

## Data layout

```
<data-dir>/
├── config.json
├── index.db                     SQLite: notes, FTS index, embeddings, folders
├── models/needle3.cact
└── notes/<folder>/<slug>-<id>.md
```

Notes are Markdown with frontmatter (`id`, `title`, `folder`, `created`,
`updated`, `tags`, `source`). SQLite is the query index; the Markdown files are
the durable record.

## Note generation

Needle3 is a small on-device **tool-calling** model, not a writing model — it
does not rewrite prose. Notebot therefore splits the work:

- **Body** — your original text, cleaned deterministically (whitespace,
  capitalization, terminal punctuation).
- **Title** — suggested by the model, with a derived fallback.
- **Folder** — the model's suggestion, falling back to keyword routing
  (`poems`, `books`, `shopping`, `work`, `ideas`) and then `inbox`.

## Development

```bash
make web                             # rebuild the dashboard
make worker && make nb && make daemon
go test ./... -short                 # test suite
go vet ./cmd/... ./internal/...
gofmt -w cmd internal
```

`web/dist` is built by `make web` and embedded into `notebotd` by
`web/embed.go` — there is no copy step. The built assets are **not** committed;
a tracked `web/dist/.gitkeep` placeholder keeps `go build ./...` working before
the first build, and the daemon serves a "dashboard not built" page until you
run `make web` and rebuild notebotd.
