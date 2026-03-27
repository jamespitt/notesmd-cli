# notesmd-cli HTTP API

The `serve` command starts an HTTP API server that provides read and write access to an Obsidian vault — tasks, notes, and projects — without requiring Obsidian to be running.

## Starting the server

```bash
notesmd-cli serve
notesmd-cli serve --port 8080
notesmd-cli serve --vault "My Vault"
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--port`, `-p` | `7070` | Port to listen on |
| `--vault`, `-v` | (default vault) | Vault name; uses the configured default if omitted |

The server binds to all interfaces (`0.0.0.0`) and includes permissive CORS headers, making it accessible from any origin on the local network.

---

## Configuration

The server reads settings from `~/.config/notesmd-cli/config.json`:

```json
{
  "default_vault_name": "My Vault",
  "default_task_folders": ["Tasks", "Daily"],
  "default_projects_folder": "Projects"
}
```

| Key | Default | Description |
|-----|---------|-------------|
| `default_vault_name` | — | Which vault to use when `--vault` is not passed |
| `default_task_folders` | (whole vault) | Folders to scan for tasks; scans entire vault if empty |
| `default_projects_folder` | `"Projects"` | Folder that contains project subdirectories |
| `default_calendar_folder` | `"Journal/Calendar"` | Folder containing calendar event files; tasks from here are returned with `type: "event"` |

---

## Response format

All responses are JSON. Successful responses return HTTP `200 OK` (or `201 Created` for new resources). Errors return a JSON object with an `"error"` key:

```json
{ "error": "description of what went wrong" }
```

---

## Notes API

### `GET /api/notes`

List all note paths in the vault.

**Response:**
```json
{ "notes": ["folder/note.md", "other.md"] }
```

---

### `GET /api/notes/{path}`

Retrieve the content and parsed frontmatter of a note. The path is relative to the vault root; `.md` extension is optional.

**Response:**
```json
{
  "path": "folder/note.md",
  "content": "---\ntitle: Example\n---\nBody text",
  "body": "Body text",
  "frontmatter": { "title": "Example" }
}
```

---

### `POST /api/notes/{path}`

Create or update a note.

**Body:**
```json
{
  "content": "Note body here",
  "overwrite": false,
  "append": false
}
```

| Field | Type | Description |
|-------|------|-------------|
| `content` | string | Text to write |
| `overwrite` | bool | Replace the note if it already exists |
| `append` | bool | Append `content` to the end of an existing note |

**Response:** `201 Created`
```json
{ "path": "folder/note.md" }
```

---

### `PATCH /api/notes/{path}`

Modify a note in place.

**Move a note:**
```json
{ "action": "move", "newPath": "new/location/note" }
```
Updates all `[[wikilinks]]` that reference the old path.

**Set a frontmatter key:**
```json
{ "action": "set", "key": "status", "value": "In Progress" }
```

**Delete a frontmatter key:**
```json
{ "action": "delete", "key": "status" }
```

**Response:**
```json
{ "path": "folder/note.md", "frontmatter": { "status": "In Progress" } }
```

---

### `DELETE /api/notes/{path}`

Permanently delete a note from the vault.

**Response:**
```json
{ "deleted": "folder/note.md" }
```

---

### `GET /api/search?q={term}`

Full-text search across all notes in the vault.

**Response:**
```json
{
  "results": [
    { "path": "folder/note.md", "line": 12, "snippet": "...matching line text..." }
  ]
}
```

---

## Tasks API

Tasks are Obsidian markdown checkbox items. The server scans the configured `default_task_folders` (or the whole vault if none are set) on every request — there is no caching.

### Task object

```json
{
  "file_path": "Tasks/Work.md",
  "line_num": 14,
  "title": "09:30-10:30 Team standup",
  "status": "todo",
  "type": "task",
  "due": "2026-04-01",
  "scheduled": "2026-03-27T09:30",
  "priority": "high",
  "repeat": "weekly",
  "tags": ["Work", "Today"],
  "level": 0,
  "list_name": "Work",
  "start_time": "09:30",
  "end_time": "10:30",
  "google_id": "UUdOdWVWUkVTX2I1SkJQVg"
}
```

| Field | Description |
|-------|-------------|
| `file_path` | Vault-relative path to the source file |
| `line_num` | 1-based line number in the file (used for all write operations) |
| `title` | Raw task title as written in the file |
| `status` | `"todo"` or `"completed"` |
| `type` | `"task"` (regular task) or `"event"` (from the configured calendar folder) |
| `due` | From `[due::YYYY-MM-DD]` or `📅 YYYY-MM-DD` |
| `scheduled` | From `[scheduled::YYYY-MM-DD]` or `[scheduled::YYYY-MM-DDTHH:MM]` |
| `priority` | From `[priority::high\|medium\|low]` |
| `repeat` | From `[repeat::...]` |
| `tags` | `#Tag` values (without `#`) |
| `level` | Indentation level (0 = top-level) |
| `list_name` | File stem of the source file (e.g. `Work.md` → `"Work"`) |
| `start_time` | Parsed from `HH:MM` or `HH:MM-HH:MM` prefix in the title |
| `end_time` | Parsed from `HH:MM-HH:MM` prefix in the title |
| `google_id` | From `[google_id::...]`; used as the stable unique identifier for calendar events |

---

### `GET /api/tasks`

All tasks across all configured task folders (both complete and incomplete).

**Response:**
```json
{ "tasks": [ /* task objects */ ] }
```

---

### `GET /api/tasks/today`

Incomplete tasks that match any of:
- `scheduled` date == today
- `due` date == today
- Tagged `#Today` (case-insensitive)

---

### `GET /api/tasks/tomorrow`

Incomplete tasks where `scheduled` or `due` date == tomorrow.

---

### `GET /api/tasks/overdue`

Incomplete tasks where `due` date is strictly before today.

---

### `GET /api/tasks/timeline`

Incomplete timed tasks for today, sorted chronologically by start time. A task is included if it has both `start_time` and `end_time` (i.e. a `HH:MM-HH:MM` prefix in the title) and is "today" by any of:
- `scheduled` or `due` == today
- Tagged `#Today`
- File name contains today's date (e.g. `Calendar_2026-03-27.md`)

Results include both `type: "task"` and `type: "event"` items.

---

### `GET /api/tasks/now`

Returns contextual task information based on the current time, derived from today's timed tasks:

```json
{
  "last":        { /* task object or absent */ },
  "current":     { /* task object or absent */ },
  "next":        { /* task object or absent */ },
  "second_next": { /* task object or absent */ }
}
```

Time is extracted from:
1. A `HH:MM-HH:MM` or `HH:MM` prefix in the task title
2. A time component in the `scheduled` field (`YYYY-MM-DDTHH:MM`)

If no task's time window contains the current time, `current` is absent and `next` points to the nearest upcoming task.

---

### `GET /api/tasks/lists`

Returns the sorted list of unique `list_name` values (file stems) across all tasks.

```json
{ "lists": ["Personal", "Shopping", "Work"] }
```

---

### `GET /api/tasks/list/{name}`

All tasks from the file whose stem matches `name` (e.g. `Work` → `Work.md`).

```json
{ "tasks": [ /* task objects */ ] }
```

---

### `POST /api/tasks/list/{name}`

Append a new incomplete task to the named list file.

**Body:**
```json
{ "title": "Write up meeting notes" }
```

**Response:** `201 Created`
```json
{ "list": "Work", "title": "Write up meeting notes" }
```

---

### `PATCH /api/tasks/{path}`

Modify a task in place. The `path` is the vault-relative file path (`.md` extension optional). All actions require either `line` (the 1-based line number from `line_num`) **or** `google_id` (preferred for calendar events). If both are provided, `line` takes precedence.

**Toggle status:**
```json
{ "line": 14, "status": "completed" }
{ "google_id": "UUdOdWVWUkVTX2I1SkJQVg", "status": "todo" }
```

**Rename:**
```json
{ "action": "rename", "line": 14, "title": "New task title" }
```

**Set due date:**
```json
{ "action": "set-due", "line": 14, "due": "2026-04-01" }
```
Replaces any existing `[due::...]` or legacy `📅` due date. Appends `[due::2026-04-01]` if none existed.

**Set scheduled date/time:**
```json
{ "action": "schedule", "line": 14, "scheduled": "2026-03-28T09:30" }
{ "action": "schedule", "line": 14, "scheduled": "2026-03-28" }
```
Replaces any existing `[scheduled::...]`.

**Move to another list:**
```json
{ "action": "move", "line": 14, "new_list": "Personal" }
```
Removes the task line from the source file and appends it to the destination list file (looked up by name within the configured task folders).

**Response** (all actions): HTTP `200` with the updated field values echoed back.

---

### `DELETE /api/tasks/{path}`

Remove a task line from its file. Accepts either `line` or `google_id`.

**Body:**
```json
{ "line": 14 }
{ "google_id": "UUdOdWVWUkVTX2I1SkJQVg" }
```

**Response:**
```json
{ "path": "Tasks/Work.md", "line": 14 }
```

---

## Projects API

A project is a subdirectory inside the configured `default_projects_folder` (default: `"Projects"`) that contains a `.md` file sharing the directory name, with `tags: Project` in its YAML frontmatter.

**Example structure:**
```
Projects/
  Center Parcs Trip/
    Center Parcs Trip.md    ← must have tags: Project
    Budget.md
    Packing list.md
```

**Example frontmatter:**
```yaml
---
tags: Project
status: In Progress
deadline: 2026-08-15
goal: Plan the family holiday
---
```

### Project object

```json
{
  "name": "Center Parcs Trip",
  "title": "Center Parcs Trip",
  "status": "In Progress",
  "deadline": "2026-08-15",
  "goal": "Plan the family holiday",
  "dir_path": "Projects/Center Parcs Trip"
}
```

---

### `GET /api/projects`

List all detected projects.

```json
{ "projects": [ /* project objects */ ] }
```

---

### `GET /api/projects/{name}`

Return a project's metadata and all its associated tasks.

Tasks are gathered from:
1. All `.md` files within the project's directory
2. Tasks in the configured task folders whose title contains `[[{name}]]`

Duplicates are removed.

```json
{
  "project": { /* project object */ },
  "tasks": [ /* task objects */ ]
}
```

---

### `POST /api/projects/{name}/tasks`

Append a new incomplete task to the project's main `.md` file.

**Body:**
```json
{ "title": "Book accommodation" }
```

**Response:** `201 Created`
```json
{ "project": "Center Parcs Trip", "title": "Book accommodation" }
```
