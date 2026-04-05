# Security Model

## Phase 2 Scope

Phase 2 adds remote connectivity, but keeps the remote security model deliberately narrow:

- strict SSH host key verification
- profile-based connection metadata
- explicit auth mode selection
- no secret storage backend yet
- no password auth yet

The goal is to make remote browsing safe enough to use without pulling later-phase secret management and transfer orchestration into the codebase prematurely.

## Current Guarantees

### Explicit filesystem boundaries

- The TUI operates on typed `fs.URI` values instead of opaque path strings.
- Filesystem capabilities gate mutation operations centrally.
- Read-only adapters still reject writes at the adapter boundary.

### Strict host key verification

- SFTP connections use `knownhosts.New(...)` from `golang.org/x/crypto/ssh/knownhosts`.
- The default host key database is `~/.ssh/known_hosts`.
- A profile or manual connect form can override that path with `known_hosts_file`.
- If the host key is missing or mismatched, the connection fails immediately.

There is currently no trust-on-first-use flow, no "accept once" prompt, and no silent host key enrollment.

### Profile storage is non-secret

`profiles.toml` stores connection metadata only:

- host
- port
- user
- path
- auth mode
- identity file path
- known hosts file path

It must not store plaintext passwords. The current docs and examples are written to reinforce that boundary.

### Supported auth modes

Phase 2 supports:

- `agent`
- `key`

`agent` uses `SSH_AUTH_SOCK`.

`key` uses a private key file referenced by `identity_file`.

Passphrase-protected keys are intentionally not handled through an in-app prompt in this phase. The expected workflow is to unlock them through `ssh-agent`.

## Current Non-Goals

The following are intentionally not implemented yet:

- password auth
- secret storage backends
- macOS Keychain integration
- Linux Secret Service integration
- Windows Credential Manager integration
- encrypted fallback secret store
- audit logging

These remain Phase 5 or later concerns.

## User-Facing Failure Modes

Remote connections fail fast for:

- missing `known_hosts` file
- unknown remote host key
- mismatched remote host key
- missing SSH agent when `auth = "agent"`
- missing or invalid private key when `auth = "key"`
- passphrase-protected private keys when no external agent is being used

This bias toward explicit failure is intentional. MiddayCommander should not silently downgrade verification or invent ad-hoc trust decisions.

## Transfer Boundary

Although the SFTP adapter can now perform direct remote writes such as `mkdir`, `rename`, `delete`, and file creation, transfer flows are still held back:

- local <-> remote copy is not available
- local <-> remote move is not available
- queue-backed overwrite policy is not available
- checksum verification is not available

This boundary protects the app from accidental "half-built" transfer semantics before the Phase 3 transfer manager exists.

## Bookmark and URI Persistence

- bookmarks store canonical URIs instead of adapter-specific path assumptions
- remote bookmarks therefore persist as explicit `sftp://...` targets
- bookmark persistence does not store credentials

This keeps persisted navigation state inspectable and non-secret.

## Follow-on Work

Later phases will add:

- secret storage providers
- optional password workflows
- audit logging
- transfer verification
- retry policy for remote transfers
- richer host key UX if needed
