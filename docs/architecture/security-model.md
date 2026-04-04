# Security Model

## Phase 1 Scope
Phase 1 does not add remote connectivity or secret storage yet. The security work in this phase is limited to keeping the filesystem boundary explicit and avoiding hidden path assumptions in the UI.

## Current Guarantees
- The TUI operates on typed `fs.URI` values instead of passing opaque local paths between packages.
- Filesystem capabilities gate mutation operations so read-only adapters can reject writes centrally.
- Bookmark persistence stores canonical URI strings and migrates legacy local-path records on load.

## Deferred Security Work
The following items are intentionally deferred to later phases:

- SSH host key verification
- `known_hosts` support
- password and key storage providers
- audit logging
- transfer verification and retry policy
