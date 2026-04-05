# Remote Filesystem Architecture

## Phase 2 Scope

Phase 2 extends the Phase 1 filesystem abstraction with native SFTP-backed remote browsing and direct remote mutations. The supported schemes are now:

- `file`: local filesystem access
- `archive`: read-only browsing inside supported archive files
- `sftp`: remote browsing and direct remote file operations over SSH/SFTP

This phase is intentionally limited to connection management, browsing, and direct remote mutations. It does not introduce the queue-based transfer engine yet.

## Core Model

- `internal/fs/filesystem.go` defines the shared `URI`, `Entry`, and `FileSystem` contracts.
- `internal/fs/capabilities.go` defines capability flags for list/read/write/mkdir/rename/remove.
- `internal/fs/router.go` resolves a URI to the correct adapter and centralizes navigation and mutation dispatch.

The UI works only with typed `fs.URI` and `fs.Entry` values. It never performs adapter-specific path math directly.

## Modules Involved

### Filesystem adapters

- `internal/fs/local`
- `internal/fs/archive`
- `internal/fs/sftp`

### Remote connection support

- `internal/profiles`
- `internal/fs/sftp/options.go`
- `internal/fs/sftp/hostkeys.go`
- `internal/fs/sftp/auth.go`
- `internal/fs/sftp/client.go`
- `internal/fs/sftp/pool.go`

### TUI integration

- `internal/tui/dialogs/profiles.go`
- `internal/tui/dialogs/connect.go`
- `internal/app/app.go`

## URI Model

### Local

`file` URIs keep absolute local paths in `URI.Path`.

### Archive

`archive` URIs use:

- `URI.Path` for the backing archive file path
- `URI.Query["entry"]` for the in-archive path

### SFTP

`sftp` URIs use:

- `URI.Host`
- `URI.Port`
- `URI.User`
- `URI.Path`
- selected query values:
  - `auth`
  - `identity_file`
  - `known_hosts_file`

Canonical examples:

```text
sftp://nexus@192.168.1.30/home/nexus?auth=agent
sftp://deploy@files.example.com:2222/srv/releases?auth=key&identity_file=~/.ssh/id_ed25519&known_hosts_file=~/.ssh/known_hosts
```

## Connection Flow

There are three entry points into a remote location:

1. `Ctrl-K` / `Shift-F2` opens the profiles overlay
2. the profiles overlay can open the manual connect dialog with `n`
3. `Ctrl-G` can still accept a raw `sftp://...` URI

The flow is:

1. TUI collects a profile or manual connect form
2. profile/manual input is normalized into `profiles.Profile`
3. `internal/fs/sftp/options.go` converts that profile into canonical SFTP options and URI
4. the active panel receives the `sftp` URI
5. the router resolves the URI to `internal/fs/sftp`
6. the SFTP adapter asks its connection pool for a reusable client
7. the pool opens or reuses a strict SSH/SFTP session
8. panel navigation proceeds through shared `List`, `Stat`, `Parent`, and `Join`

## Connection Reuse

`internal/fs/sftp/pool.go` caches SSH/SFTP clients by normalized connection target. Calls that differ only by remote path reuse the same underlying transport.

The cache key includes:

- host
- port
- user
- auth mode
- identity file
- known hosts file

This allows one session to serve repeated `List`, `Stat`, `OpenReader`, and direct mutation calls while a panel remains active on the same remote endpoint.

`app.Model.Close()` closes the router, which closes the SFTP adapter, which closes the pooled SSH/SFTP clients.

## Navigation Rules

- local directories are navigated with `file` URIs
- archive entry/exit behavior from Phase 1 is preserved
- remote directories are navigated with `sftp` URIs
- `Parent()` on an SFTP URI walks the remote path hierarchy and clamps at `/`
- bookmarks remain URI-based, so remote locations reuse the same bookmark flow as local ones

## Mutation Rules

### Available in Phase 2

The SFTP adapter now supports direct remote mutations through the shared filesystem interface:

- `Mkdir`
- `Rename`
- `Remove`
- `OpenWriter`

This lets existing UI flows such as `F7`, rename, and delete work on remote panels without special-case TUI logic.

### Still deferred

The transfer engine is still not present, so operations that depend on cross-panel transfer orchestration remain out of scope:

- local -> remote copy
- remote -> local copy
- local -> remote move
- remote -> local move
- remote -> remote copy across endpoints

`internal/actions/transfer_guard.go` intentionally blocks `F5` and `F6` whenever SFTP is involved so the Phase 2 scope line stays explicit.

## Rename Boundaries

SFTP rename is supported only within the same normalized remote endpoint. The adapter rejects rename attempts across different SFTP hosts, ports, users, or auth contexts.

That keeps `Rename` aligned with the semantics of a direct filesystem rename instead of quietly turning it into a transfer operation.

## Atomic Write Strategy

Phase 2 uses a best-effort atomic write path for SFTP writes:

1. write to a sibling temporary file
2. attempt `PosixRename`
3. fall back to replace-by-rename when possible

This is intentionally smaller in scope than the planned Phase 4 transfer verification and retry pipeline, but it already gives remote direct writes a safer default behavior than truncating in place.

## Follow-on Work

Later phases will build on this foundation with:

- queue-backed transfer scheduling
- transfer progress and Bubble Tea events
- overwrite and conflict policies
- copy verification and retries
- audit logging
- secrets storage backends
