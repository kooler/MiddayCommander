# Transfer Engine

## Current Status

The transfer engine is now present as an initial production-oriented slice under `internal/transfer`.

It is already responsible for:

- queueing transfer jobs
- running them in the background
- emitting progress snapshots to the Bubble Tea app
- handling local <-> SFTP copy and move flows
- applying conflict policy before writes
- applying post-copy verification
- retrying failed jobs with short backoff
- recording transfer lifecycle events through `internal/audit`

This is still not the final Phase 3/4 implementation, but the branch has moved past the earlier "SFTP transfers are blocked" boundary.

## Module Boundary

### Engine

- `internal/transfer/types.go`
- `internal/transfer/manager.go`

### TUI integration

- `internal/tui/dialogs/transfer.go`
- `internal/tui/dialogs/transfer_options.go`
- `internal/app/app.go`
- `internal/app/commands.go`

### Supporting packages

- `internal/audit`
- `internal/secrets`

## Transfer Flow

For transfers that stay entirely local, the existing direct action path is still used.

For transfers that involve SFTP, the flow is now:

1. the user presses `F5` or `F6`
2. the standard confirm dialog opens
3. the app opens `Transfer Options`
4. the user chooses:
   - conflict policy
   - verification mode
   - retry count
5. the app submits a normalized `transfer.Request`
6. `internal/transfer.Manager` enqueues the job
7. the worker executes the job through `internal/fs.Router`
8. progress snapshots are emitted back to the TUI
9. recent completion/failure state is retained in the overlay

## Job Model

Each queued job currently carries:

- operation: `copy` or `move`
- source URIs
- destination directory URI
- conflict policy
- verify mode
- retry count

Current conflict policies:

- `overwrite`
- `skip`
- `rename`

The engine still understands `ask`, but the UI does not expose it yet because there is not yet a mid-transfer interactive resolution loop.

Current verify modes:

- `none`
- `size`
- `sha256`

## Retry Behavior

Retries are automatic and per-job.

- the first execution is attempt `1`
- each configured retry adds one more possible attempt
- the manager waits for a short increasing backoff before retrying
- retry state is visible in the transfer overlay
- retry lifecycle is also written to the audit log

Retries currently focus on making transient remote failures less disruptive. They are not yet exposed as a fully featured queue-control system with pause/cancel/resume semantics.

## Copy / Move Semantics

### Copy

- files are streamed through the shared filesystem interface
- the writer uses best-effort atomic output when the destination adapter supports it
- destination conflict handling runs before opening the writer
- verification runs after the copy completes

### Move

- same-filesystem rename is attempted first when possible
- otherwise move falls back to:
  1. copy
  2. verify
  3. delete source

This makes cross-filesystem move behave like a safe composed operation instead of assuming rename semantics across endpoints.

## Progress Model

The transfer overlay currently shows:

- current job
- queued jobs
- recent jobs
- file-count progress
- byte-count progress
- retry attempt count

This is enough for visibility during local <-> remote transfers, but it is still intentionally lighter than a full queue-management UI.

## Audit Integration

`internal/audit` currently writes JSONL events to `~/.config/mdc/audit.log`.

The transfer manager records:

- queued
- started
- retrying
- completed
- failed

This is foundational logging, not yet the final audit UX.

## Current Limitations

The current transfer slice still leaves room for later work:

- no pause / resume / cancel controls
- no priority reordering
- no interactive conflict prompts while a job is running
- no user-selectable retry backoff policy
- no transfer-history browser in the TUI
- no resumable partial-file transport

## Follow-on Work

Later phases can build on the current engine with:

- richer queue controls
- better conflict resolution UX
- more durable retry policy
- stronger verification policy defaults
- audit browsing and filtering
- secret-provider-backed remote credentials
