# Transfer Engine

## Current Status

The queue-based transfer engine is still not implemented.

Phase 1 moved file operations onto shared `fs.URI` and router-based dispatch. Phase 2 adds native SFTP browsing and direct remote mutations, but still stops short of full transfer orchestration.

## What Exists Today

Current file-operation behavior is split into two groups.

### Direct filesystem mutations

These run directly through the active filesystem adapter:

- local mkdir / rename / delete
- remote SFTP mkdir / rename / delete
- local file writes
- direct remote file writes used by the SFTP adapter

These are not transfer-engine features. They are direct mutations on a single filesystem endpoint.

### Transfer-style operations

These still flow through `internal/actions`:

- copy
- move

For now:

- local <-> local copy and move work
- archive sources remain read-only
- any copy or move that involves SFTP is blocked intentionally

This is enforced by `internal/actions/transfer_guard.go`.

## Why SFTP Transfers Are Deferred

Cross-filesystem transfers need more than raw read/write support. They also need:

- queue management
- progress reporting
- overwrite policy handling
- recursive scheduling
- recovery and retry behavior
- optional verification
- UI that explains what is in flight

Phase 2 avoids shipping partial remote transfer behavior that would be hard to trust or reason about.

## Present UX Boundary

In the current build:

- remote panels can browse SFTP targets
- remote panels can create directories
- remote panels can rename entries
- remote panels can delete entries
- `F5` and `F6` are rejected when SFTP is involved

The error is intentional and marks the boundary between "filesystem operations" and the not-yet-landed transfer engine.

## Planned Phase 3 Direction

Phase 3 is expected to add:

- queue-backed transfer scheduling
- background workers
- progress events for Bubble Tea
- local <-> remote copy
- remote-aware move behavior
- overwrite/conflict policy handling

Later phases can then layer on:

- verification modes
- audit logging
- retries
- more durable transfer semantics
