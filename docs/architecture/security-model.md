# Security Model

## Phase 2 Scope

The remote security model is still conservative, but the branch has now moved beyond basic Phase 2 browsing.

The current implementation includes:

- strict SSH host key verification
- profile-based connection metadata
- explicit auth mode selection
- queue-backed remote transfer execution
- transfer verification modes
- JSONL audit logging
- encrypted file fallback secret storage
- no password auth yet

The goal remains the same: add remote access and transfer capability without silently weakening SSH trust or storing plaintext secrets.

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

### Transfer verification

Remote-capable transfers now support:

- `none`
- `size`
- `sha256`

Verification is configured before the job is queued and runs after the destination write completes.

### Audit logging

Transfer lifecycle events are written as JSONL records to:

```text
~/.config/mdc/audit.log
```

This is foundational logging for queued remote transfer activity. It is not yet the final reporting or browsing UX.

### Secrets scaffolding

The codebase now includes the `internal/secrets.Provider` abstraction and an encrypted file fallback backend written to:

```text
~/.config/mdc/secrets.json
```

This is not yet the final credential-storage story. It exists so later platform-native providers can plug into a stable boundary.

## Current Non-Goals

The following are intentionally not implemented yet:

- password auth
- secret storage backends
- macOS Keychain integration
- Linux Secret Service integration
- Windows Credential Manager integration
- full audit UI
- transfer pause / resume / cancel UX

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

Although the SFTP adapter already supported direct remote writes, transfer flows now also exist through `internal/transfer`:

- local <-> remote copy
- local <-> remote move
- queue-backed progress reporting
- selectable conflict policy
- selectable verification mode
- short automatic retry

What still remains intentionally incomplete:

- interactive conflict prompts during a running job
- richer retry policy controls
- platform-backed credential storage
- password-auth workflows

This keeps the current branch usable without pretending the later phases are finished.

## Bookmark and URI Persistence

- bookmarks store canonical URIs instead of adapter-specific path assumptions
- remote bookmarks therefore persist as explicit `sftp://...` targets
- bookmark persistence does not store credentials

This keeps persisted navigation state inspectable and non-secret.

## Follow-on Work

Later phases will add:

- secret storage providers
- optional password workflows
- audit browsing UX
- richer retry policy
- richer host key UX if needed
