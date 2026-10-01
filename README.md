# Notebot

Local CLI + daemon + dashboard for Needle3-refined notes.

## Layout

- `needle/` — Go binding for Needle3 (do not modify API).
- `models/needle3.cact` — dev model archive (read-only source, never moved).
- `cmd/nb` — CLI capture tool.
- `cmd/notebotd` — local daemon (HTTP API + embedded dashboard).
- `internal/store` — Markdown + SQLite storage, `config.json`.
- `internal/agent` — Needle3 toolset + refinement loop.
- `internal/server` — HTTP routes + auth.
- `web/` — React/Shadcn dashboard (builds to `web/dist`).

## Model location

Dev uses `./models/needle3.cact` directly. Prod resolves `model_path`
from `config.json` (default `$XDG_DATA_HOME/notebot/models/needle3.cact`,
fallback `~/.notebot/models/needle3.cact`). Copy once:

```
mkdir -p ~/.notebot/models
cp models/needle3.cact ~/.notebot/models/
```

Or run `nb setup` to copy it and set the dashboard password.

## How notes are produced

Needle3 is a small on-device **tool-calling** model, not a writing model: it
cannot rewrite or expand prose (it echoes input verbatim). So `nb` splits the
work:

- **Body** — your original text, cleaned deterministically (whitespace,
  capitalization, terminal punctuation).
- **Title / folder** — suggested by Needle3; folder falls back to keyword
  routing (`poems`, `books`, `shopping`, `work`, `ideas`) when the model is
  unsure.

The Agent sidebar uses Needle3 for real tool calls (create/move/delete notes
and folders). Destructive calls require confirmation.

## Env overrides

- `NEEDLE_ENGINE` — engine .so path
- `NEEDLE_WORKER` — needle-worker binary path
- `NEEDLE_MODEL` — weights path override
- `NOTEBOT_PORT` — daemon port (default 8765)
- `NOTEBOT_DATA` — data dir override

## Build

```
make web      # builds the dashboard into web/dist (embedded into notebotd)
make worker && make nb && make daemon
make install  # installs nb + notebotd to ~/.local/bin
go test ./... -short
```

`web/dist` is the single source of truth for the frontend and is embedded
directly by the `web` package (`web/embed.go`) — there is no copy step. It is
committed so `go build ./...` works without Node. After editing the UI, run
`make web` and rebuild the daemon.

## Usage

Capture (quoted or bare words both work):

```
nb "I will go to Nilkhet to buy books next saturday"
nb today i wanted to write a poem
nb "deadline 20 october" --dry-run
```

Query:

```
nb list                      # all notes (alias: ls)
nb list work                 # notes in a folder
nb list --since yesterday --until now
nb list --since 2026-09-01 --until 2026-09-30
nb list --since "3 days ago"          # today, yesterday, 3 days ago, 2026-01-02
nb today / nb yesterday      # notes created today / yesterday
nb find milk                 # full-text search
nb find poem --since "1 week ago"   # search within a time range
nb show <id>                 # print one note (id prefix is enough)
```

Manage:

```
nb edit <id> --title "New title" --folder work
nb edit <id>                 # opens $EDITOR (title/folder/---/body format)
nb mv <id> work              # move to a folder
nb rm <id> -y                # delete (asks first without -y)
nb mkfolder recipes
nb rename old new            # rename a folder
nb rmfolder recipes -y       # delete folder; its notes move to inbox
nb folders
```

Natural language (best-effort):

```
nb ask "move the milk note to shopping"
nb ask "delete the notebot deadline note"
nb ask "rename the weekend folder to errands"
nb ask -y "create a note about the standup"
```

Explicit verbs (move/delete/rename folder) are routed deterministically in Go;
other requests fall through to Needle3 tool-calling.

Daemon + dashboard:

```
notebotd --set-password
notebotd   # serves login + dashboard at http://127.0.0.1:8765/
```
