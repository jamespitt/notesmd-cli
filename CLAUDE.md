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
mocks/         → Test doubles for all interfaces
```

Each command in `cmd/` calls a corresponding action in `pkg/actions/`. Actions accept interfaces (not concrete types) for testability.

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

## Task Format

Tasks in Obsidian notes use Markdown checkbox syntax with optional metadata fields:

```markdown
- [ ] Task description #Tag [scheduled::2026-02-18T09:30] [google_id::UUdOdWVWUkVTX2I1SkJQVg]
- [x] Completed task #Tag
```

**Fields:**
- `[ ]` / `[x]` — Incomplete / complete status
- `#Tag` — Tag(s) e.g. `#Today`, `#Tomorrow`
- `[scheduled::datetime]` — Scheduled datetime in ISO 8601 format (e.g. `2026-02-18T09:30`)
- `[google_id::base64string]` — Google Calendar event ID
- Metadata fields follow the pattern `[key::value]`

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

- **CLI config**: `~/.config/notesmd-cli/config.json` (stores default vault)
- **Obsidian config**: Read from Obsidian's native `config.json` (read-only)

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
