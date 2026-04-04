# ADR 0001: Filesystem Abstraction

## Status
Accepted

## Context
MiddayCommander started as a local-first file manager with archive browsing layered on top of direct path handling inside the UI. That made the panel state, file operations, and bookmark persistence depend on raw local paths and package-specific behavior. The planned remote filesystem work requires a stable abstraction that the TUI can use without knowing whether a location is local, inside an archive, or remote.

## Decision
- Replace `internal/vfs` with a new `internal/fs` package centered on `URI`, `Entry`, `FileSystem`, and a router that dispatches by scheme.
- Make the panel and app layers carry `fs.URI` and `fs.Entry` instead of local path strings and `fs.DirEntry`.
- Represent local locations as `file` URIs with absolute filesystem paths in `URI.Path`.
- Represent archive locations as `archive` URIs with the backing archive path in `URI.Path` and the in-archive location in `URI.Query["entry"]`.
- Keep archive support read-only in Phase 1 and route mutations through capability checks.
- Move bookmark persistence to URI strings so future remote locations can be stored without another migration.

## Consequences
- The TUI no longer owns path math for parent/join/clean behavior.
- Local and archive browsing share one navigation model.
- File actions can target any readable or writable adapter supported by the router.
- Existing bookmark data needs a one-time compatibility path from legacy `path` records to canonical URI strings.
- Future SFTP work can add a new adapter under `internal/fs/sftp` without rewriting panel state again.
