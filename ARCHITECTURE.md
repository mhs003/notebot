# Architecture

Notebot is a local-only note system: a CLI that captures short thoughts, an
on-device model that titles and files them, and a password-protected web
dashboard served from a daemon on `127.0.0.1`. It is a single Go module with an
embedded React frontend.

```
┌─────────────┐   POST /api/capture        ┌──────────────────────────────┐
│  nb (CLI)   │───────────────────────────▶│        notebotd              │
│             │◀── note JSON ──────────────│  (single Needle instance)    │
│  offline?   │                            │                              │
│  in-process │                            │  ┌────────┐   ┌───────────┐  │
│  fallback   │                            │  │ agent  │──▶│  needle   │  │
└─────────────┘                            │  └───┬────┘   │  worker   │  │
                                           │      │        │ (subproc) │  │
┌─────────────┐   /api/*  (session cookie) │      ▼        └───────────┘  │
│  Browser    │───────────────────────────▶│  ┌────────┐                  │
│  dashboard  │◀── SPA from web/dist ──────│  │ store  │                  │
└─────────────┘                            │  └───┬────┘                  │
                                           └──────┼───────────────────────┘
                                                  ▼
                                    Markdown notes + SQLite index
```

## Repository layout

| Path | Purpose |
| --- | --- |
| `needle/` | Vendored Go binding for the Needle3 engine. Treated as a third-party library. |
| `models/needle3.cact` | Model weights (git-ignored; copied into the data dir by `nb setup`). |
| `internal/store` | Persistence: Markdown files, SQLite index, config, date queries. |
| `internal/agent` | Needle3 toolset, capture cleanup, intent router, confirmation gate. |
| `internal/dates` | Natural-language time parsing for `--since` / `--until`. |
| `internal/server` | HTTP API, session auth, SPA file serving. |
| `cmd/nb` | The CLI. |
| `cmd/notebotd` | The daemon. |
| `web/` | React + Tailwind dashboard source, and `web/embed.go` which embeds `web/dist`. |
| `deploy/` | systemd unit for the daemon. |

## Data model

### On disk

The data directory is `$NOTEBOT_DATA`, else `$XDG_DATA_HOME/notebot`, else
`~/.notebot` (see `store.DefaultDataDir`).

```
<data-dir>/
├── config.json                 0600 — port, data_dir, model_path,
│                                     password_hash, session_secret
├── index.db                    SQLite
├── models/needle3.cact         weights
└── notes/<folder>/<slug>-<id>.md
```

Notes are stored as Markdown with YAML-ish frontmatter so they are readable and
greppable outside the app:

```
---
id: 85ca051983292253
title: "Book shopping at Nilkhet"
folder: shopping
created: 2026-10-01T08:43:17Z
updated: 2026-10-01T08:43:17Z
tags: ["books"]
source: cli
raw_prompt: "I will go to Nilkhet ..."
---
I will go to Nilkhet to buy some books on 21 November.
```

`source` is one of `cli`, `web`, `agent`.

### SQLite schema (`store.migrate`)

- `notes(id PK, title, folder, body, tags, source, raw_prompt, created, updated, path)`
- `notes_fts` — FTS5 virtual table over `id, title, body, folder`
- `embeddings(doc_id PK, dim, vec)` — little-endian float32 blob; cosine is computed in Go
- `folders(name PK, created)` — folders that exist independently of any note

`created` / `updated` are RFC3339 UTC strings; because that format sorts
lexicographically, range filters are plain string comparisons.

### Folders

Folders are the union of `folders` rows and `DISTINCT notes.folder`. Creating a
note implicitly registers its folder. Deleting a folder moves its notes to
`inbox` rather than destroying them.

## Components

### `internal/store`

Owns all persistence. The exported surface is deliberately small:

- Read: `Get`, `List`, `Recent`, `Folders`, `Search`, `Query`, `SearchQuery`, `ResolveID`
- Write: `Create`, `CreateFolder`, `Update`, `Delete`, `DeleteFolder`, `Move`, `RenameFolder`
- Vectors: `SaveEmbedding`, `LoadEmbedding`, `Cosine`

`Query`/`SearchQuery` take a `Query{Folder, Since, Until, ByUpdated, Limit,
Oldest}`. `SearchQuery` prefers FTS5 and falls back to a `LIKE` scan when the
MATCH expression is rejected, so odd input still returns results.
`ResolveID` accepts a unique ID prefix — this is why the CLI can print 8-char
IDs and still round-trip them.

### `internal/agent`

Wraps one `needle.Needle` and exposes three operations:

- `RefineAndRoute(ctx, raw)` → `Refined{Title, Body, Folder, Tags}`
- `Capture(ctx, raw)` → creates a note from `RefineAndRoute`
- `RunStep(ctx, prompt, confirmID)` → `StepResult` for the agent surfaces

What the model actually does is constrained by a hard finding: **Needle3 cannot
rewrite prose.** With no tools it returns no calls; with a "reply" tool it
copies the input verbatim. So the design splits the work:

- **Body** is cleaned deterministically in `refine.go` (`cleanText`): whitespace
  collapsed, spacing before punctuation fixed, first letter capitalized, terminal
  punctuation ensured. The model's `content` field is never trusted.
- **Title** comes from the model when usable, else `deriveTitle` (first sentence,
  trimmed to 60 chars and stripped of trailing stopwords).
- **Folder** uses the model's hint, else `routeFolder`, which prefers an existing
  folder named in the text and then a keyword table
  (`poems`, `books`, `shopping`, `work`, `ideas`), defaulting to `inbox`.

**Tool selection.** `route.go` resolves explicit verbs deterministically in Go —
`move`, `delete`/`remove`, and `rename folder` — by matching note titles and
folder names. This exists because the model reliably picks the wrong tool for
these (it will `create_note` when asked to move). Everything else falls through
to the model via `withContext`, which prepends candidate notes **only when the
request references an existing note** — dumping all notes into a plain create
request made the model copy unrelated bodies.

**Confirmation gate.** `delete_note`, `move_note`, and `rename_folder` are
destructive. `RunStep` refuses to execute them directly: it stores the call in a
package-level `pending` map under a random `confirm_id` and returns
`{needs_confirm, confirm_id, summary}`. The caller re-invokes with `confirm_id`
to actually apply it. Only the first destructive call in a turn is queued.

**Concurrency.** One `Needle` owns one worker subprocess with one conversation.
`Agent.mu` serializes every model call. A cancelled context kills the worker and
loses the conversation (the engine has no interrupt), so calls use generous
timeouts rather than short ones.

### `internal/dates`

`Range(now, preset, since, until)` plus `ParseInstant`. Understands `now`,
`today`, `yesterday`, `tomorrow`, `N minutes/hours/days/weeks ago`, and several
date layouts (`2006-01-02`, RFC3339, `2 january 2006`, …). Day-granular
expressions resolve to the start or end of the day depending on whether they are
used as a lower or upper bound.

### `internal/server`

Plain `net/http` with `http.ServeMux`; no framework.

| Route | Notes |
| --- | --- |
| `GET /api/health` | open |
| `POST /api/auth/login` | open; sets the session cookie |
| `GET /api/auth/status` | open; reports `authed` and `password_set` |
| `GET/POST /api/notes` | list / create |
| `GET/PUT/PATCH/DELETE /api/notes/{id}` | single note |
| `GET/POST /api/folders` | list / create |
| `GET /api/recent`, `GET /api/search?q=` | |
| `GET/PUT /api/settings` | model path, port (attached with the agent routes) |
| `POST /api/capture` | CLI capture (attached with the agent routes) |
| `POST /api/agent` | agent tool calls (attached with the agent routes) |

`AttachAgentRoutes` is separate so the daemon can inject the live `Agent` while
`server` stays free of an agent dependency.

**Auth.** The password is bcrypt-hashed in `config.json`. The session cookie is
not a random per-login token but a constant derived from a persistent secret:
`HMAC-SHA256(session_secret, "notebot-session-v1")`. That is what makes logins
survive page refreshes and daemon restarts. Logout rotates the secret in memory.
If `password_hash` is empty, the daemon runs open (first-run convenience).
The CLI authenticates to the daemon by computing the same HMAC from `config.json`.

**Static files.** `WebFS` is an `fs.FS` set by the daemon from the embedded
`web.Dist`. Unknown paths fall back to `index.html` (SPA routing). When `WebFS`
is nil the daemon serves a "dashboard not built" placeholder instead of failing.

### `cmd/nb`

Argument handling has two paths. If `os.Args[1]` is not a known command, the
whole line is treated as a capture — this is what makes `nb today i wanted to
write a poem` work without quotes during capture. Otherwise Cobra dispatches to
a subcommand.

Capture prefers the daemon (`POST /api/capture`) and falls back to running the
agent in-process so the CLI works offline.

Commands, grouped:

- Capture: `nb <text...>` (`--dry-run`, `--folder`)
- Query: `list`/`ls`, `find`, `show`, `today`, `yesterday`, `recent`, `search`
- Manage: `edit`, `mv`, `rm`, `mkfolder`, `rmfolder`, `rename`
- Agent: `ask` (routes locally first, then the daemon, then in-process)
- Setup: `setup`, `config get|set`, `folders`

Destructive commands prompt unless `-y`.

### `cmd/notebotd`

Loads config, ensures a session secret, opens the store, builds one `Agent`
(the expensive model load happens once here), sets `server.WebFS`, and serves on
`127.0.0.1:<port>`. `--set-password` sets the bcrypt hash and exits.

The daemon finds `needle-worker` beside its own executable and then on `$PATH`,
so all three binaries must be installed together (`make install`).

Under systemd, the data directory is resolved from the process environment —
`NOTEBOT_DATA`, else `$XDG_DATA_HOME/notebot`, else `~/.notebot`. systemd does
not inherit the shell's exports, so the unit must set `NOTEBOT_DATA` explicitly
to the directory `nb setup` used. `nb install-service` generates the unit with
the correct absolute paths, the resolved data dir, and the configured port.

### `web/`

Vite + React + Tailwind, styled after shadcn/ui (components are hand-written
copies in `web/src/components/ui`). Views: Login, Home/Recent/Folders grid,
note editor, Settings. Floating `Create` opens a dialog; floating `Agent` opens
a right-hand sheet that shows tool calls and renders a Confirm button when the
server returns `needs_confirm`.

`web/dist` is built by `make web` and embedded by `web/embed.go`. There is no
copy step, because `go:embed` cannot reference paths outside its own package —
hence a `web` package rather than embedding from `cmd/notebotd`. `web/dist` is
committed so `go build ./...` works without a Node toolchain.

## Key flows

**Capture.** `nb "text"` → daemon `/api/capture` (or in-process) →
`agent.Capture` → `RefineAndRoute` (clean body, model title/folder, keyword
fallback) → `store.Create` writes Markdown and indexes it.

**Query.** `nb list --since yesterday` → `dates.Range` → `store.Query` → rows.
`nb find milk` → `store.SearchQuery` → FTS5, `LIKE` fallback.

**Agent.** `nb ask "delete the milk note"` → `agent.route` matches the verb and
resolves the note → returns a destructive call → CLI prompts → re-invokes with
`confirm_id` → `store.Delete`.

**Login.** `POST /api/auth/login` compares bcrypt → sets
`notebot_session = HMAC(secret)`. Every later `/api/*` request is checked with
`hmac.Equal`.

## Constraints and trade-offs

- **The model is small and literal.** It routes and classifies; it does not
  write. Anything resembling rewriting is done in Go. When adding model-facing
  features, assume it will pick the wrong tool for explicit verbs and prefer a
  deterministic path.
- **The tool schema is part of the static prefix.** More tools means slower
  startup (~2.5 s with 5 tools, ~12 s with 60). `tools.json` currently has 8
  tools; keep it small.
- **One worker, one conversation.** All model calls are serialized; the daemon
  is not designed for concurrent generations.
- **Linux x86_64 only.** The vendored engine is `linux-x86_64`; other platforms
  fail with a clear message.
- **Markdown + SQLite dual store.** SQLite is the query index, Markdown is the
  durable, human-readable record. They are written together and can be rebuilt
  from the files if needed.
- **Localhost only.** The daemon binds `127.0.0.1`; there is no TLS or
  multi-user support, by design.

## Extension points

- **New agent tool** — add it to `internal/agent/tools.json`, handle it in
  `Agent.exec`, and add it to `isDestructive` if it mutates existing data.
- **New query filter** — extend `store.Query` and add flags in
  `cmd/nb/query.go`.
- **New time expression** — add a case to `internal/dates`.
- **New dashboard view** — add a component under `web/src`, wire it into
  `Sidebar` and `App`, then run `make web` and rebuild the daemon.
