# Transfer Engine

## Current Status
The transfer engine is not implemented in Phase 1.

Phase 1 only refactors MiddayCommander so file operations run through the shared filesystem abstraction and router. That work is the prerequisite for the queue-based transfer manager planned for later phases.

## Phase 1 Implications
- Copy, move, delete, mkdir, and rename now operate on `fs.URI` values.
- Archive writes remain blocked by filesystem capabilities.
- There is no background queue, conflict policy UI, checksum verification, or progress dialog integration yet.

## Planned Follow-on
Later phases will add:

- queue-backed transfer scheduling
- per-transfer progress events for Bubble Tea
- cross-filesystem copy and move semantics
- overwrite and conflict policies
- optional verification passes
