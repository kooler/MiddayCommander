# Remote Filesystem Architecture

## Phase 1 Scope
Phase 1 introduces the abstraction boundary required for remote filesystems without adding SSH or SFTP yet. The supported schemes are:

- `file`: local filesystem access
- `archive`: read-only browsing inside supported archive files

## Core Model
- `internal/fs/filesystem.go` defines the shared `URI`, `Entry`, and `FileSystem` contracts.
- `internal/fs/capabilities.go` defines capability flags used to gate list/read/write/mkdir/rename/remove behavior.
- `internal/fs/router.go` resolves a URI to its adapter and centralizes dispatch for list, stat, open, join, parent, clean, rename, and remove.

## Adapters
- `internal/fs/local` provides full local read/write access.
- `internal/fs/archive` provides read-only access to files and directories inside archives.

The TUI is intentionally isolated from adapter-specific rules. It only stores the current `fs.URI`, renders `fs.Entry`, and asks the router to navigate.

## Navigation Rules
- Local directories are navigated with `file` URIs.
- Entering an archive converts the selected file URI into an `archive` URI rooted at `entry="."`.
- `Parent()` on an archive URI walks up within the archive until the archive root, then returns the containing local directory as a `file` URI.
- The panel still restores the cursor to the archive filename when leaving an archive.

## Mutation Rules
- Write operations are routed through `internal/actions` and the router rather than calling `os.*` directly from the app.
- Archive mutations are rejected by capability checks in Phase 1.
- External edit/view and fuzzy find remain local-only in Phase 1.

## Follow-on Work
This design is the base for future:

- native SFTP adapters
- profile-backed remote connections
- transfer queue orchestration
- audit logging
- secret storage integration
