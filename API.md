# notesmd-cli HTTP API

The `serve` command starts an HTTP API server that provides read and write access to an Obsidian vault — tasks, notes, and projects — without requiring Obsidian to be running.

## Starting the server

```bash
notesmd-cli serve
notesmd-cli serve --port 8080
notesmd-cli serve --vault "My Vault"
notesmd-cli serve --vault personal --vault work   # a specific set of vaults
```

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--port`, `-p` | `7070` | Port to listen on |
| `--vault`, `-v` | (the configured vaults, else the default vault) | Vault to serve: a configured vault id, an Obsidian vault name, or an absolute path. Repeatable |

The server binds to all interfaces (`0.0.0.0`) and includes permissive CORS headers, making it accessible from any origin on the local network.

---

## Configuration

The server reads settings from `~/.config/notesmd-cli/preferences.json`:

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
| `default_task_folders` | (whole vault) | Folders to scan for tasks; scans entire vault if empty. An entry ending in `.md` (e.g. `"Action Items.md"`) is a single file rather than a folder |
| `default_projects_folder` | `"Projects"` | Folder that contains project subdirectories |
| `default_calendar_folder` | `"Journal/Calendar"` | Folder containing calendar event files; tasks from here are returned with `type: "event"` |

---

## Multiple vaults

One server can serve several vaults (personal, work, …) and clients switch between them per request. Add a `vaults` array to `preferences.json`:

```json
{
  "default_vault_name": "james_notes",
  "default_task_folders": ["Tasks", "Journal"],
  "vaults": [
    {
      "id": "personal",
      "label": "Personal",
      "path": "/home/james/src/james_notes"
    },
    {
      "id": "work",
      "label": "Work",
      "path": "/home/james/src/tm_notes",
      "task_folders": ["Action Items.md", "Key Action Items.md", "Projects", "Journal"],
      "projects_folder": "Projects",
      "calendar_folder": "Journal/Calendar"
    }
  ]
}
```

| Key | Description |
|-----|-------------|
| `id` | What clients pass as `?vault=<id>`. Required |
| `label` | Display name for vault pickers; defaults to `id` |
| `path` | An Obsidian vault name or an absolute path (a path needs no Obsidian config entry). Required |
| `task_folders` | Per-vault task folders; falls back to `default_task_folders` when omitted |
| `projects_folder` | Per-vault projects folder; falls back to `default_projects_folder` |
| `calendar_folder` | Per-vault calendar folder; falls back to `default_calendar_folder` |

**The first entry is the default vault** — the one used by requests that name no vault, which is what keeps single-vault clients working. `notesmd-cli vaults` prints the configured list.

**Selecting a vault:** add `?vault=<id>` to any request, or send an `X-Vault: <id>` header (the query parameter wins). An unknown id returns `404` rather than silently falling back to the default — a mistyped vault must not write into the wrong one.

```bash
curl localhost:7070/api/tasks/today?vault=work
curl -H 'X-Vault: work' localhost:7070/api/tasks/today
```

Server-side state that isn't stored in the vault (the hidden-events list) is kept per vault. With no `vaults` configured, everything behaves exactly as it did before: one vault, reachable as the default and as the id `default`.

### `GET /api/vaults`

The vaults this server offers, and which one the request resolved to.

**Response:**
```json
{
  "vaults": [
    { "id": "personal", "label": "Personal", "default": true },
    { "id": "work", "label": "Work" }
  ],
  "active": "personal"
}
```

A client can treat a `404` here as "this server predates vault switching" and hide its picker.

---

## Response format

Every endpoint below accepts the `?vault=<id>` parameter (or `X-Vault` header) described above; requests that omit it use the default vault.

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

Retrieve the content and parsed frontmatter of a note. The path is relative to the vault root; `.md` extension is optional. If `{path}` doesn't match a file exactly, it falls back to a basename match anywhere in the vault - so a bare note title (e.g. the target of an Obsidian `[[wikilink]]`) resolves the same way Obsidian itself would, without the caller needing to know the note's folder. `404` if nothing matches.

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

**Replace the whole file content** (frontmatter included) - used for plain markdown-body editing, e.g. task-front-end's linked-note editor:
```json
{ "action": "setContent", "content": "# My Note\n\nBody text." }
```

**Response:**
```json
{ "path": "folder/note.md", "frontmatter": { "status": "In Progress" } }
```
(`setContent` responds with just `{ "path": "..." }` - there's no frontmatter re-parse since the caller already has the exact content it wrote.)

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

Tasks are Obsidian markdown checkbox items. The server scans the request's vault over its configured task folders (`task_folders` for that vault, else `default_task_folders`, else the whole vault) on every request — there is no caching.

**Cancelled tasks are hidden.** A `- [-]` line, and an open task tagged `#Delete`, is a *cancelled* task: it stays in the file until the sync purges it a day or more later (see `DELETE /api/tasks` below), and no task endpoint returns it.

**Write safety.** Every task write (`PATCH`, `POST`, `DELETE`, project task creation) takes the shared vault lock — the same exclusive `flock` the Google/Todoist sync and the git auto-commit job use — and replaces the file atomically. If the lock can't be had within 3 minutes (a long sync run), the request fails with `500` and `{"error": "vault busy: timed out …"}`; retry it. The lock path is `$TASK_VAULT_LOCK`, default `~/.local/state/task_system/vault.lock`.

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
  "parent_id": "Tasks/Work.md:12",
  "list_name": "Work",
  "start_time": "09:30",
  "end_time": "10:30",
  "google_id": "UUdOdWVWUkVTX2I1SkJQVg",
  "created": "2026-09-22",
  "source": "wiki/meetings/2026-09-22 Slack Activity Summary.md",
  "user": "James Pitt, Olha Yeremenko"
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
| `parent_id` | `"file_path:line_num"` of the parent task, absent for top-level tasks. Recomputed fresh on every parse - same as `line_num` itself, it goes stale the instant a line above it is added or removed, until the next fetch. Matches a task's own identity, i.e. `parent_id === "{file_path}:{line_num}"` of another task in the same response. |
| `list_name` | File stem of the source file (e.g. `Work.md` → `"Work"`) |
| `start_time` | Parsed from `HH:MM` or `HH:MM-HH:MM` prefix in the title |
| `end_time` | Parsed from `HH:MM-HH:MM` prefix in the title |
| `google_id` | From `[google_id::...]`; used as the stable unique identifier for calendar events |
| `created` | From `[created::...]`, as written: a date (`2026-09-22`) or a timestamp (`2026-09-22 00:00:00+00:00`, what the sync writes). Omitted when absent |
| `source` | From `[source::...]`: the note the task came from (a vault-relative path, as the meeting ingest writes it). Omitted when absent |
| `user` | From `[user::...]`: the people involved, comma-separated (`"James Pitt, Olha Yeremenko"`). Omitted when absent |

| `subtasks` | Only on `GET /api/tasks/kanban` cards: the card's descendants, in file order (see that endpoint). Omitted elsewhere and when there are none |

Field names in the file are case-insensitive (`[Source::…]` and `[source::…]` are the same field).

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

### `GET /api/tasks/kanban`

Optional `?columns=Backlog,Review,Done` (comma-separated, `#` optional) swaps in a custom column set for this request, the way a per-board column list does in the `obsidian-kanban` plugin; omitted means the default three. Each column must be a valid single tag (letters, digits, `_`, `/`), otherwise `400`.

Tasks carrying one of the Kanban status tags (`ToDo`, `InProgress`, `Done` - case-insensitive), sorted column-major (all `ToDo`, then all `InProgress`, then all `Done`). Unlike every other view, completed tasks are **not** excluded - a `Done` card is normally also completed, since moving a card onto Done via `PATCH .../set-status-tag` checks it off too.

```json
{ "tasks": [ /* task objects */ ] }
```

**Subtasks are folded into their parent card.** A task with an ancestor that is itself on the board is *not* returned as a card; it is listed in that ancestor's `subtasks` array instead (the topmost on-board ancestor owns every descendant below it, whatever their own tags). A task whose ancestors are all off the board stays a card of its own, so a tagged subtask under an untagged parent never disappears from the board. `subtasks` appears only on this endpoint:

```json
{
  "title": "Self contained releases", "tags": ["InProgress"], "line_num": 56, "level": 0,
  "subtasks": [
    { "line_num": 57, "title": "Complete E2E testing pipeline integration", "status": "todo", "level": 1 },
    { "line_num": 58, "title": "Identify and engage target users", "status": "completed", "level": 1 }
  ]
}
```

`level` is relative to the card (1 = direct child, 2 = grandchild); `line_num` is in the card's file. Every other task endpoint still returns subtasks as ordinary flat tasks with `level` and `parent_id`.

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

Append a new task to the named list file.

**Body:**
```json
{ "title": "Write up meeting notes" }
{ "title": "Ship it #Done", "status": "completed" }
```

`title` is the raw text after the checkbox, so it may already carry `#tags` and `[key::value]` fields. `status` is optional: `"todo"` (default) writes `- [ ]`, `"completed"` writes `- [x]`; anything else is a `400`. An older server ignores `status` and always writes `- [ ]`.

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
Removes the task line - along with any of its existing children (contiguous lines indented deeper than it) - from the source file and appends the whole block to the destination list file (looked up by name within the configured task folders), preserving indentation relative to the moved parent. `line` must be the parent's own line; moving a child mid-tree brings only *its* descendants, not its siblings.

**Set Kanban status:**
```json
{ "action": "set-status-tag", "line": 14, "kanban_status": "InProgress" }
{ "action": "set-status-tag", "line": 14, "kanban_status": "" }
```
Replaces any existing `ToDo`/`InProgress`/`Done` tag with the given one (every other tag is untouched); `kanban_status` must be one of those three or `""` to take the task off the board. Also keeps completion in sync: setting `"Done"` checks the task off, anything else (including `""`) un-checks it.

An optional `"kanban_columns": ["Backlog", "Review", "Done"]` uses a custom column set instead: `kanban_status` must then be one of those (or `""`), and *every* one of them is stripped from the line before the new tag is added. Completion sync is unchanged: only a column named `Done` (any case) checks the task off, and moving onto anything else un-checks it.

**Set tags:**
```json
{ "action": "set-tags", "line": 14, "tags": ["groceries", "urgent"] }
```
Replaces the task's entire tag set with the given list, in order (an empty array removes all tags). The title and every `[key::value]` field are left untouched. Unlike `set-status-tag`, this never touches completion status - it's a plain tag edit.

**Edit (multiple fields at once):**
```json
{
  "action": "edit", "line": 14,
  "title": "New title", "due": "2026-04-01", "scheduled": "",
  "priority": "high", "repeat": "every day",
  "tags": ["groceries", "urgent"], "new_list": "Work"
}
```
Applies any combination of field updates in a single rewrite. **A field key omitted from the JSON body is left completely unchanged; an empty string clears it** (e.g. `"scheduled": ""` above removes the scheduled date while due/priority/repeat are set). `tags`, when present, replaces the entire tag set the same as `set-tags` (omit it to leave tags untouched). `new_list`, when present and different from the task's current list, moves the task there as a follow-up step (same mechanics as the `move` action) after the field edit is written. Every other `[key::value]` field on the line not covered above (e.g. `google_id`, custom fields) is preserved regardless.

**Add a subtask:**
```json
{ "action": "add-subtask", "line": 14, "title": "New subtask" }
```
`line` is the **parent** task's line. Inserts a new incomplete task indented one level deeper than the parent, positioned after any of the parent's existing children (so it becomes the last child) - in the same file. Nesting is still purely inferred from indentation on disk; `parent_id` in the Task object (see above) is a read-time convenience computed from that indentation, not a separate stored concept, so a subtask only needs correct indentation to be recognized as a child - and to get the right `parent_id` - on the next read.

**Make a task a subtask of another (or top level):**
```json
{ "action": "set-parent", "line": 14, "parent_line": 3 }
{ "action": "set-parent", "line": 14, "parent_line": 3, "parent_path": "Tasks/Home.md" }
{ "action": "set-parent", "line": 14, "parent_line": 0 }
```
Moves the task **and its own subtasks** so it becomes the *last* subtask of the task at `parent_line`, indented one level under it (every nested level shifts with it). Without `parent_path` the parent is in the same file; with it (a vault-relative path, `.md` optional) the parent is in that file and the task moves there - a subtask has to live in its parent's list. `parent_line: 0` promotes the task to top level, placed right after the subtree of its top-level ancestor (a task that is already top level is left alone). Refused with `400` if the target isn't a task line, or is the task itself or one of its own subtasks; `404` if `parent_path` doesn't exist; `400` if it's outside the vault. **Line numbers shift**, so the response reports where the task ended up and clients should refetch:
```json
{ "path": "Tasks/Home.md", "line": 3 }
```
Nesting is purely indentation on disk, so the change is one atomic rewrite under the vault lock; when two files are involved the destination is written before the source is trimmed, so a failure can duplicate the task but never lose it. The Google/Todoist sync then propagates the new parent (and, for a cross-file move, the new list) on its next run.

**Response** (all other actions): HTTP `200` with the updated field values echoed back.

---

### `DELETE /api/tasks/{path}`

Delete a task. Accepts either `line` or `google_id`.

A task that carries a `google_id` or `todoist_id` is **cancelled, not removed**: its line is rewritten in place as `- [-] ...`. The sync (`tasks/src/sync.py`) leaves cancelled tasks alone for at least 24h, then deletes them on Google/Todoist and moves the line to the completed archive (see `tasks/sync.md`, "Cancelled tasks"). This is deliberate: a line that just disappears looks the same to the sync as one lost in a bad merge. A task with no sync id has nothing to purge, so its line is removed as before.

Cancelled tasks (`[-]`) and open tasks tagged `#Delete` are hidden from every task endpoint. To undo a cancel within the grace period, edit the marker back to `[ ]` (and remove any `#Delete` tag) in Obsidian.

**Body:**
```json
{ "line": 14 }
{ "google_id": "UUdOdWVWUkVTX2I1SkJQVg" }
```

**Response** (`cancelled` is `true` when the line was kept as `[-]`, `false` when it was removed):
```json
{ "path": "Tasks/Work.md", "line": 14, "cancelled": true }
```

---

## Hidden calendar events

Calendar events imported into the vault can be hidden from every task view without editing the file. The list is server-side state (not stored in the vault) and is **kept per vault** — an event id only means something inside the vault it came from. `parseTasks` filters hidden events out of every task endpoint.

### `GET /api/tasks/hidden`

**Response:**
```json
{
  "events": [
    { "event_id": "3f7b…", "title": "Standup", "hidden_at": "2026-09-15T21:04:11+01:00" }
  ]
}
```

### `POST /api/tasks/hidden`

Hide an event (idempotent — hiding an already-hidden event changes nothing).

**Body:**
```json
{ "event_id": "3f7b…", "title": "Standup" }
```

**Response:** the full updated `{ "events": [...] }` list.

### `DELETE /api/tasks/hidden/{event_id}`

Unhide an event. **Response:** the full updated `{ "events": [...] }` list.

---

## Journal API

### `GET /api/journal/today/diary`

The bullet lines under the `#### Diary Notes` heading in today's daily note. The note's folder and filename format come from the vault's own Obsidian daily-notes settings (falling back to `YYYY-MM-DD` at the vault root).

**Response:**
```json
{ "entries": ["Walked the dog", "Fixed the tap"] }
```

An absent daily note is not an error — `entries` is empty.

### `POST /api/journal/today/diary`

Append a bullet to that section, creating the daily note (with minimal frontmatter and a `#### Diary Notes` heading) and/or the section if either is missing.

**Body:**
```json
{ "text": "Walked the dog" }
```

**Response:** the full updated `{ "entries": [...] }` list.

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

---

### `GET /api/projects/{name}/pages`

Every markdown file inside the project's directory, recursed into subfolders
(dot-directories skipped). Includes the main `{name}.md` and `Diary.md`.

**Response:**
```json
{
  "pages": [
    { "path": "Projects/Center Parcs Trip/Budget.md", "name": "Budget", "rel": "Budget.md" },
    { "path": "Projects/Center Parcs Trip/Notes/Packing.md", "name": "Notes/Packing", "rel": "Notes/Packing.md" }
  ]
}
```

| Field | Description |
|-------|-------------|
| `path` | Vault-relative path - use it with `GET`/`PATCH /api/notes/{path}` to read/edit the page |
| `name` | Display name: the relative path without the `.md` extension |
| `rel` | Path relative to the project directory |

To create a new page, `POST /api/notes/Projects/{name}/New Page.md`.

---

### `GET /api/projects/{name}/diary`

Raw markdown of the project's `Diary.md`.

**Response:**
```json
{ "path": "Projects/Center Parcs Trip/Diary.md", "content": "---\n...\n# ... Diary\n\n### 2026-09-06\n...", "exists": true }
```
`exists` is `false` (and `content` empty) until the first entry is added.

---

### `POST /api/projects/{name}/diary`

Append a dated entry to the project's `Diary.md`.

**Body:**
```json
{ "text": "Booked the ferry, waiting on confirmation." }
```

The entry is placed under a `### YYYY-MM-DD` heading for today. If that heading
already exists the text is appended as a new paragraph at the end of that day's
section; otherwise a new day section is inserted directly below the `# ... Diary`
title, above older days. The file (with frontmatter, `tags: ProjectDiary`, and a
title heading) is created if it doesn't exist.

**Response:**
```json
{ "path": "Projects/Center Parcs Trip/Diary.md", "content": "...full updated file..." }
```

---

## WhatsApp API

Two read-only pass-throughs to a [zapmeow](https://github.com/jamespitt/zapmeow) instance, so a client can read WhatsApp history from the same origin as the task API. They're vault-independent — the `?vault=` parameter is accepted but has no effect. The target is `http://localhost:8900` instance `1` by default, overridable with the `ZAPMEOW_URL` and `ZAPMEOW_INSTANCE_ID` environment variables. An unreachable zapmeow returns `502`; otherwise zapmeow's own JSON and status code are streamed back unchanged.

### `GET /api/whatsapp/messages?chat=&limit=&before=`

Messages, newest first. `chat` scopes to one chat JID (omit for all chats), `limit` caps the page size, `before` is a unix-seconds cursor for the next page.

### `GET /api/whatsapp/chats`

One summary row per chat (group or person), newest activity first.
