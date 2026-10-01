# AGENTS.md

Guidance for AI coding agents working in this repository. Read
`ARCHITECTURE.md` for the design; this file is the practical rulebook.

## What this project is

Notebot is a local-only note system:

- `nb` — a CLI that captures short thoughts.
- `notebotd` — a daemon that holds one Needle3 model, exposes a REST API, and
  serves a password-protected web dashboard on `127.0.0.1:8765`.
- `web/` — the React + Tailwind dashboard, embedded into `notebotd`.

Go module: `github.com/mhs003/notebot`. Go 1.22. `needle/` is vendored and
treated as third-party.

## Commands

```bash
make worker && make nb && make daemon   # build binaries into bin/
make web                                # build the dashboard into web/dist
make install                            # install nb, notebotd, needle-worker to ~/.local/bin
go test ./... -short                    # full test suite
go vet ./cmd/... ./internal/...
gofmt -w cmd internal                   # format (run before committing)
```

Quick manual smoke test:

```bash
NOTEBOT_DATA=$PWD/.tmp-nb ./bin/nb setup
NOTEBOT_DATA=$PWD/.tmp-nb ./bin/nb "deadline for the project is 20 october"
NOTEBOT_DATA=$PWD/.tmp-nb ./bin/nb list
```

## Ground rules

1. **Keep the CLI and the web dashboard in sync.** They are two front ends over
   the same store and agent. When you add or change a feature, or fix a bug, in
   either one, check whether the other needs the same change and make it in the
   same piece of work. Concretely: a new note/folder action needs both a `nb`
   subcommand and an API route the dashboard calls; new behaviour in `nb`
   (duplicate handling, confirmations, filters) needs the equivalent in the web
   UI, and vice versa. Prefer putting the logic in `internal/` (store, agent,
   server) so both front ends share it rather than reimplementing it.
   If a capability genuinely cannot or should not exist in one of them, say so
   explicitly instead of leaving it silently missing.
2. **Do not change the `needle/` public API.** `Config`, `Needle`, `Result`,
   `FunctionCall` are a stable surface. `needle/BINDINGS.md` is the reference for
   the engine; read it before touching anything under `needle/`.
3. **Do not recreate the module path.** It is `github.com/mhs003/notebot`.
   `github.com/mhs003/needless` is a *different* project and must not appear.
4. **`web/dist` is embedded but not committed.** Only the tracked
   `web/dist/.gitkeep` placeholder is in git, so `go:embed all:dist` still
   resolves on a fresh clone before any frontend build. Never commit the built
   assets. After any frontend change run `make web` and rebuild the daemon, or
   the binary keeps serving stale assets. `go:embed` cannot reference parent
   directories, which is why the embed lives in `web/embed.go` and not in
   `cmd/notebotd`.
5. **Never break session persistence.** Sessions are derived from
   `config.json`'s `session_secret` via `store.SessionToken`, not stored in
   memory. Reverting to an in-memory session map reintroduces "logged out on
   every refresh".
6. **Return `[]`, not `null`, for empty lists in the API.** The dashboard maps
   over these; JSON `null` crashes React and blanks the page.
7. **Match the existing style.** Doc comments on packages and exported
   identifiers; no comments narrating obvious code. Wrap errors with context
   (`fmt.Errorf("store: ...: %w", err)`). Standard library HTTP, no web
   framework. No cgo — SQLite is `modernc.org/sqlite`.

## Facts about Needle3 that shape the code

These were established by experiment and are load-bearing. Do not "simplify"
them away.

- **The model cannot rewrite text.** With no tools it returns zero calls; with a
  "reply" tool it copies the input verbatim. Note bodies are therefore cleaned
  deterministically in `internal/agent/refine.go`; the model's `content` field is
  discarded. Do not wire the model in as a rewriter.
- **The model picks the wrong tool for explicit verbs.** Asked to move or delete
  a note it will often emit `create_note`; asked to "move the meeting to friday"
  it emitted a bogus `rename_folder`. `internal/agent/route.go` resolves `move`,
  `delete`/`remove`, `rename folder`, and `update`/`change` deterministically in
  Go before the model is consulted. New mutating verbs belong there.
- **Capture only routes when the target resolves.** `nb "<text>"` treats text as
  a command only when the first word is a management verb *and*
  `agent.RoutePrompt` finds the note or folder. Otherwise it captures. Never let
  command-shaped text fall through to the model — it will invent a call.
- **Never silently create a duplicate title.** When a capture's refined title
  matches an existing note, the daemon returns `needs_choice` and the client
  asks whether to update or create. Do not remove that prompt or "simplify" it
  into a warning; a second note with the same title is the failure this prevents.
- **Context is injected only when a note is referenced.** `withContext` prepends
  candidate notes only when the request matches an existing title. Dumping all
  notes into a plain create request makes the model copy unrelated bodies into
  the new note.
- **One worker, one conversation, serialized.** `Agent.mu` guards all model
  calls. Cancelling a model call's context kills the worker and loses the
  conversation — prefer generous timeouts over cancellation.
- **The tool schema is part of the static prefix.** Startup cost grows with the
  number of tools. `internal/agent/tools.json` has 8 tools; keep it small.
- **Engine support is `linux-x86_64` only.**

## Where to make common changes

| Task | Files |
| --- | --- |
| New CLI command | `cmd/nb/*.go` and add the name to `knownCommands` in `cmd/nb/main.go` |
| New agent tool | `internal/agent/tools.json`, `Agent.exec`, `isDestructive` if it mutates |
| New deterministic verb routing | `internal/agent/route.go` (+ test in `route_test.go`) |
| New command verb for `nb "<text>"` | `commandVerbs` in `cmd/nb/ask.go` — keep it in sync with `route.go` |
| New query filter | `internal/store/query.go`, flags in `cmd/nb/query.go` |
| New time expression | `internal/dates/dates.go` (+ test) |
| New API endpoint | `internal/server/handlers.go` or `agent_routes.go` |
| New dashboard view | `web/src/`, then `Sidebar.jsx` + `App.jsx` |

The CLI and the dashboard are paired front ends — see ground rule 1. A change to
one is usually a change to the other:

| Capability | CLI | Web |
| --- | --- | --- |
| Capture with duplicate choice | `cmd/nb/capture_choice.go`, `/api/capture` `decision` | `CreateModal.jsx`, `AgentSheet.jsx` choice panel |
| Time filtering | `--since/--until`, `today`, `yesterday` | `VIEW_PARAMS` in `App.jsx`, `/api/notes?since=&until=` |
| Search | `nb find` | search box → `/api/search` |
| Move note | `nb mv` | folder field in `NoteView.jsx` |
| Folder management | `mkfolder`/`rename`/`rmfolder` | `FolderCard.jsx`, `/api/folders/{name}` |
| Destructive confirmation | `confirm()` in `cmd/nb` | confirm UI in `AgentSheet.jsx` |

## Pitfalls

- `knownCommands` in `cmd/nb/main.go` gates the capture fast path: any first
  argument that is not a known command is captured as note text. A new
  subcommand that is missing from that map becomes unreachable.
- The CLI authenticates to the daemon by computing the HMAC session token from
  `config.json`; if you change the cookie scheme, update
  `cmd/nb/main.go:tryDaemonCapture` and `cmd/nb/ask.go` too.
- `config.json` is written `0600` and holds the password hash — keep it that way.
- `models/` is excluded via `.git/info/exclude`; the model is copied into the
  data dir by `nb setup`, never committed.
- Folder deletion moves notes to `inbox`; it must never delete notes.
- `notebotd` finds `needle-worker` beside its own executable, then on `$PATH`.
  `make install` must therefore ship all three binaries; installing only `nb`
  and `notebotd` leaves the daemon unable to start.
- The systemd unit must pin `NOTEBOT_DATA` to the resolved data dir. systemd
  does not inherit the shell's exports, so a unit that omits it (or hardcodes a
  different path) makes the daemon look in the wrong directory. `nb
  install-service` writes it correctly; do not hand-edit a hardcoded path back in.

## Definition of done

- `gofmt -w cmd internal` run.
- `go test ./... -short` and `go vet ./cmd/... ./internal/...` pass.
- `go build ./...` succeeds.
- New behaviour verified by actually running it (`nb ...`, or `curl` against a
  daemon started with `NOTEBOT_PORT` and `NOTEBOT_DATA` pointing at a throwaway
  directory), not merely by reading the code.
- Clean up throwaway data directories (`.tmp-nb`, `.nb-*`) and stop any daemon
  you started.
