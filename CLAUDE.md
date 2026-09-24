# CLAUDE.md

## Project Overview

**notesmd-cli** is a Go CLI tool for interacting with Obsidian vaults from the terminal without requiring Obsidian to be running. Originally named obsidian-cli, renamed to avoid confusion with the official Obsidian CLI.

- **Module**: `github.com/Yakitrak/notesmd-cli`
- **Go version**: 1.19+
- **CLI framework**: [Cobra](https://github.com/spf13/cobra)

## Architecture

Layered architecture with dependency injection:

```
cmd/           → CLI layer (Cobra commands, flag parsing)
pkg/actions/   → Business logic layer (use cases)
pkg/obsidian/  → Core domain (Vault, Note, Uri interfaces & implementations)
pkg/config/    → Configuration management (vault discovery, CLI config)
pkg/frontmatter/ → YAML frontmatter parsing/manipulation
pkg/tasks/     → Task parsing/editing (checkbox lines, tags, [key::value] fields, Kanban status, subtasks) - used by `cmd/tasks.go`, `cmd/add_task.go` and `pkg/server/`
pkg/vaultlock/ → The shared flock every task-file writer takes (see "Writing task files" below)
pkg/server/    → HTTP API server (`serve` command) - task-front-end's backend, see API.md
                 vaults.go = the multi-vault registry + per-request vault resolution
pkg/projects/  → Project note discovery, used by the server's /api/projects endpoints
mocks/         → Test doubles for all interfaces
```

Each command in `cmd/` calls a corresponding action in `pkg/actions/`. Actions accept interfaces (not concrete types) for testability. `pkg/tasks/` and `pkg/server/` are the exception - they're a separate, newer layer (no `pkg/obsidian` interfaces involved) that reads/writes task files directly via the standard library.

## Key Commands

| Command | Description |
|---------|-------------|
| `open` | Open note in Obsidian |
| `search` | Fuzzy search and open notes |
| `search-content` | Search file contents |
| `create` | Create new notes |
| `daily` | Create/open daily note |
| `move` | Move/rename notes (updates all internal links) |
| `delete` | Delete notes |
| `list` | List vault contents |
| `print` | Print note contents to terminal |
| `frontmatter`/`fm` | View/edit YAML frontmatter |
| `set-default` | Set default vault |
| `print-default` | Print default vault info |
| `tasks` | Search tasks by folder/tag/date range, print to console (`pkg/actions/tasks.go` + `pkg/obsidian/task.go` - separate, simpler parser than `pkg/tasks/`) |
| `add-task` | Append a task to a list file under the vault lock; idempotent by title (`cmd/add_task.go` → `tasks.AddTask`). Use it instead of editing synced list files directly |
| `serve` | Start the HTTP task API server (`--port`, default 7070) that `task-front-end` talks to - see below. `--vault` is repeatable; with no flag it serves every vault in the config |
| `vaults` | List the switchable vaults from the config (`vaults` array), marking the default |

## Task Format

Tasks in Obsidian notes use Markdown checkbox syntax with optional metadata fields:

```markdown
- [ ] Task description #Tag #ToDo [due::2026-02-20] [scheduled::2026-02-18T09:30] [priority::high] [repeat::every day] [google_id::UUdOdWVWUkVTX2I1SkJQVg]
- [x] Completed task #Tag #Done
```

**Fields:**
- `[ ]` / `[x]` / `[-]` — Incomplete / complete / cancelled. A cancelled task is kept in the file for the sync (`tasks/src/sync.py`) to purge after 24h. An open task tagged `#Delete` is cancelled too. `pkg/tasks.parseLine` returns nothing for either, so they are hidden from every task endpoint; `taskLineRe` still matches `[-]` so subtask block boundaries stay correct. (The separate `tasks` CLI parser in `pkg/obsidian/task.go` only matches `[ ]`/`[x]`, so it hides `[-]` too but still lists open `#Delete` tasks.)
- `#Tag` — any number of tags, all preserved (not just the first). `#ToDo`/`#InProgress`/`#Done` are reserved as Kanban status tags (`pkg/tasks.KanbanTags`) - see below.
- List membership (`list_name` in the Task object) always comes from the file a task lives in, never from a tag.
- Nesting/`parent_id` is inferred purely from indentation (4 spaces per level) when a file is parsed - there's no separate stored parent reference.
- `[due::...]`, `[scheduled::...]`, `[priority::...]`, `[repeat::...]` — standard dataview-style fields.
- `[google_id::base64string]` — Google Calendar event ID.
- Any other `[key::value]` field round-trips untouched.

Full field-by-field reference (including the HTTP Task JSON shape) is in **API.md**.

## Writing task files

Task files are also rewritten by the Python sync and a git auto-commit job, so every mutator in `pkg/tasks/` follows two rules:

- **Take the vault lock.** `vaultlock.Lock()` (an exclusive `flock` on `$TASK_VAULT_LOCK`, default `~/.local/state/task_system/vault.lock`, the same file as `tasks/src/vault_lock.py` and `tasks/git.sh`) is acquired at the top of each leaf mutator and released with `defer`. It waits up to 3 minutes and isn't reentrant, so **don't call one locked mutator from another** (`SetStatusTag` is a lock-free wrapper around `SetStatusTagIn` for this reason). A new mutator needs its own `Lock()`.
- **Write atomically.** Use `writeFileAtomic` (temp file + rename, keeps the mode, writes through symlinks), never `os.WriteFile`, for vault files.

`PATCH set-parent` goes through `tasks.SetParent` (`pkg/tasks/subtasks.go`): it moves a task's whole block (the task plus its deeper-indented task lines) under a parent, in the same file or another, or promotes it to top level; nesting is only indentation on disk, so it is a single locked rewrite (destination written before source when two files are involved). The Kanban endpoint uses `tasks.KanbanCardsIn` to fold subtasks into their parent card instead of returning them as separate cards; other endpoints stay flat. `obsidian-kanban/src/taskModel.ts` mirrors both rules in TypeScript (`foldSubtasks`, `setParentInContent`, `promoteInContent`) for its local mode - keep them in step.

`DELETE /api/tasks` goes through `tasks.CancelOrDeleteTask`: a task with a `google_id`/`todoist_id` is rewritten as `[-]` (cancel), any other task's line is removed. Tests: `pkg/vaultlock`, `pkg/tasks/atomic_test.go`, `pkg/tasks/add_test.go`.

## HTTP Task Server (`pkg/server/`, `pkg/tasks/`)

`notesmd-cli serve` starts an HTTP API over the vault's tasks - list/create/edit/delete/move tasks, a Kanban endpoint, subtasks, project notes. It's the backend `task-front-end` (the companion SvelteKit web app) talks to; `obsidian-kanban` (the companion Obsidian plugin) implements the same conventions directly against the vault instead of over HTTP. See **API.md** for the full endpoint/action reference.

### Multiple vaults

One server can serve several vaults (e.g. personal + work), declared as a `vaults` array in `preferences.json`; clients pick one per request with `?vault=<id>` (or an `X-Vault` header) and discover the list from `GET /api/vaults`. The first configured vault is the default, so a request that names none behaves exactly as a single-vault server always did.

- `pkg/server/vaults.go` holds the registry (`server.Vault`), the `withVault` middleware that resolves the id once per request into the request context, and `/api/vaults`. Handlers therefore never touch a fixed vault - they call `s.vaultOf(r)` / `s.vaultTarget(r)`, and the folder helpers (`getVaultPath`, `getTaskFolders`, `getProjectsFolder`, `getCalendarFolder`, `parseTasks`) all take the request.
- `pkg/obsidian.ConfiguredVault` adapts one config entry to `VaultManager`, with per-vault task/projects/calendar folders (work and personal vaults rarely agree on where tasks live). An empty folder field falls back to the global `default_*` preference.
- State that lives outside the vault (hidden calendar events) is namespaced by vault: the default vault keeps `hidden_events.json`, others get `hidden_events-<id>.json`.
- An unknown vault id is a `404`, never a fallback to the default - a mistyped id must not write into the wrong vault.

## Build & Test

```bash
make build-all      # Compile for all platforms
make test           # Run all tests
make test-coverage  # Generate coverage report
make release        # Full release workflow
make release-patch  # Patch version bump
```

Run a specific test:
```bash
go test ./pkg/actions/...
go test ./pkg/obsidian/...
```

## Platform Support

- **macOS**: amd64, arm64
- **Linux**: amd64, arm64 — includes Flatpak and Snap Obsidian installs
- **Windows**: amd64
- **WSL**: Detected via `/proc/sys/fs/binfmt_misc/WSLInterop`; resolves Windows paths via `cmd.exe`

## Configuration

- **CLI config**: `~/.config/notesmd-cli/preferences.json` (default vault, task/projects/calendar folders, and the `vaults` array for vault switching)
- **Obsidian config**: Read from Obsidian's native `~/.config/obsidian/obsidian.json` (read-only) - maps vault names to paths. A vault configured by absolute path needs no entry there.

## Obsidian URI Protocol

Commands interact with Obsidian via `obsidian://` URIs:
```
obsidian://open?vault=MyVault&file=Note&section=Heading
obsidian://new?vault=MyVault&file=NewNote&content=...
obsidian://daily?vault=MyVault
```

## Testing Conventions

- Tests co-located with source (`*_test.go`)
- Mock implementations in `mocks/` for all interfaces
- Pattern: Arrange → Act → Assert using `testify/assert`
- Dependency injection via interfaces enables full unit testing without filesystem/Obsidian

## Adding New Commands

1. Add command file in `cmd/` (Cobra `Command` struct, `init()` registration)
2. Add action file in `pkg/actions/` (business logic, accepts interfaces)
3. Add/extend interfaces in `pkg/obsidian/` if needed
4. Add mock in `mocks/` if new interface added
5. Add tests in `pkg/actions/` and relevant packages
