# Midday Commander

A modern dual-panel terminal file manager written in Go, inspired by Midnight Commander.

Midday Commander (`mdc`) keeps the classic commander-style workflow while adding archive browsing, fuzzy finding, themes, configurable keybindings, and now native SFTP-backed remote navigation.

![Midday Commander](images/sc_general.png)

**Bookmarks** — jump to local and remote locations you visit often
![Bookmarks](images/sc_bookmarks.png)

**Themes** — TOML-based themes with live preview
![Themes](images/sc_themes.png)

**Fuzzy Find**
![Fuzzy Find](images/sc_fzf.png)

## Features

- **Dual-panel file browsing** with independent local or remote navigation
- **Native SFTP browsing** with strict `known_hosts` verification
- **Remote profiles** loaded from `profiles.toml`
- **Remote bookmarks** stored as canonical URIs
- **Queue-backed remote transfers** with progress overlay
- **Local <-> remote copy and move** over SFTP with transfer options
- **Queue controls** for pause, resume, cancel current, and clear queued work
- **Archive browsing** for ZIP, TAR, 7z, RAR, GZ, BZ2, XZ, LZ4 and related formats
- **File operations** including copy, move, delete, rename, and mkdir
- **Remote mkdir / rename / delete** on SFTP-backed locations
- **Transfer verification** with `none`, `size`, or `sha256`
- **Automatic retry** for queued remote transfers
- **Audit logging foundation** for transfer activity
- **Configurable keybindings** via `config.toml`
- **Fuzzy finder** for fast local recursive search
- **Live theme picker** with instant preview
- **External editor/viewer** for local files through `$EDITOR` and `$PAGER`
- **Mouse support** for menu bar and panel interaction
- **Single binary** distribution

## Current Remote Scope

The current remote build goes beyond basic browsing and now includes the first transfer-manager slice.

Available now:

- connect to remote hosts over SFTP
- open remote locations from saved profiles
- open remote locations from a manual connect dialog
- bookmark remote directories
- create directories remotely
- rename remote files and directories
- delete remote files and directories
- open remote locations directly with raw `sftp://...` URIs through `Go To`
- copy local <-> remote through the transfer manager
- move local <-> remote through the transfer manager
- choose conflict policy before remote transfer starts
- choose transfer verification mode before remote transfer starts
- use short automatic retries for transient transfer failures
- review in-flight and recent transfer progress in the transfer overlay
- open the audit log from the transfer overlay without leaving the TUI

Remote fuzzy find, remote external edit, and remote external view are also still deferred. Those flows remain local-only in the current build.

Still deferred:

- richer conflict resolution UX while a job is already running
- pause / resume / cancel queue controls
- platform-native secret stores
- password-based SSH auth
- remote fuzzy find
- remote external view / edit

## Installation

### Using Homebrew

```bash
brew install kooler/apps/MiddayCommander
```

Run: `mdc`

### From releases

Download a binary from the [Releases](https://github.com/kooler/MiddayCommander/releases) page.

### Build from source

Requires Go 1.21 or later.

```bash
git clone https://github.com/kooler/MiddayCommander.git
cd MiddayCommander
make build
```

The binary will be at `./mdc`. Move it to your `$PATH`:

```bash
sudo mv mdc /usr/local/bin/
```

### Build targets

```bash
make build   # Build the binary
make run     # Build and run
make test    # Run tests
make vet     # Run go vet
make clean   # Remove binary
```

## Quick Start

```bash
mdc
```

The left panel opens in the current directory, the right panel in your home directory. Navigate with arrow keys or `j` / `k`, switch panels with `Tab`, and open directories with `Enter`.

### Open a remote location

1. Press `Ctrl-K` or `Shift-F2`.
2. If you already have remote profiles, choose one and press `Enter`.
3. Press `n` in the profiles overlay to open the manual connect dialog.
4. If no profiles exist yet, Midday Commander opens the manual connect dialog directly.

You can also jump to a raw SFTP URI with `Ctrl-G`, for example:

```text
sftp://nexus@192.168.1.30/home/nexus?auth=agent&known_hosts_file=~/.ssh/known_hosts
```

### Start a remote transfer

1. Put a local panel and a remote panel side by side.
2. Press `F5` for copy or `F6` for move.
3. Confirm the source and destination as usual.
4. In the `Transfer Options` overlay, choose:
   - conflict policy
   - verification mode
   - retry count
5. Press `Enter` to queue the job.

The transfer overlay opens automatically and keeps showing current, queued, and recent jobs.
From that overlay you can also pause/resume the queue, cancel the current job, clear queued work, and open the audit log viewer.

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `F1` | Help |
| `F2` | Bookmarks |
| `Shift-F2` | Remote connect |
| `F3` | View local file (`$PAGER`) |
| `F4` | Edit local file (`$EDITOR`) |
| `F5` | Copy to other panel |
| `F6` | Move to other panel |
| `Shift-F6` | Rename |
| `F7` | Create directory |
| `F8` | Delete |
| `F9` | Fuzzy finder |
| `F10` | Quit |
| `Esc Esc` | Quit (double-press) |
| `Tab` | Switch active panel |
| `Ctrl-U` | Swap panels |
| `Ctrl-G` | Go to path or URI |
| `Ctrl-K` | Remote connect |
| `Ctrl-P` | Fuzzy finder |
| `Ctrl-B` | Bookmarks |
| `Ctrl-T` | Theme picker |
| `Ctrl-R` | Run command in local directory |

### Navigation

| Key | Action |
|-----|--------|
| `Up` / `k` | Move cursor up |
| `Down` / `j` | Move cursor down |
| `PgUp` / `PgDn` | Page up / down |
| `Home` / `End` | Jump to first / last |
| `Enter` | Enter directory or run configured file action |
| `Space` | Run alternate configured file action |
| `Backspace` | Go to parent directory |
| Type letters | Quick search in current panel |

### Selection

| Key | Action |
|-----|--------|
| `Insert` | Toggle selection on current file |
| `Shift-Up` | Select and move up |
| `Shift-Down` | Select and move down |

### Bookmarks

| Key | Action |
|-----|--------|
| `a` | Add current location as a bookmark |
| `d` | Delete selected bookmark |
| `f` | Filter bookmarks |
| `0`-`9` | Quick jump to bookmark |
| `Enter` | Navigate to bookmark |
| `Esc` | Close |

Bookmarks are URI-based, so local, archive, and remote SFTP locations can all be stored.

### Remote Profiles Overlay

| Key | Action |
|-----|--------|
| `Up` / `Down` | Move selection |
| `f` | Filter profiles |
| `n` | Open manual connect dialog |
| `0`-`9` | Quick jump to a profile |
| `Enter` | Connect to selected profile |
| `Esc` | Close |

### Manual Connect Overlay

| Key | Action |
|-----|--------|
| `Tab` / `Shift-Tab` | Move between fields |
| `Up` / `Down` | Move between fields |
| `Left` / `Right` | Move cursor, or switch auth mode on the auth field |
| `Enter` | Connect |
| `Esc` | Close |

### Transfer Options Overlay

| Key | Action |
|-----|--------|
| `Tab` / `Shift-Tab` | Move between fields |
| `Up` / `Down` | Move between fields |
| `Left` / `Right` | Change the selected option |
| `0`-`5` | Set retry count on the retries field |
| `Enter` | Queue the transfer |
| `Esc` | Close |

### Transfers Overlay

| Key | Action |
|-----|--------|
| `Esc` / `q` | Hide the overlay |
| `p` | Pause or resume the queue |
| `c` | Cancel the current running transfer |
| `k` | Clear queued transfers that have not started yet |
| `a` | Open the audit log viewer |

The overlay shows the current transfer, queued jobs, recent outcomes, and retry attempt counts when a job is retried automatically.

### Audit Overlay

| Key | Action |
|-----|--------|
| `Up` / `Down` | Scroll |
| `PgUp` / `PgDn` | Page |
| `r` | Refresh audit entries |
| `Esc` / `q` | Close |

## Remote Access

### Remote profiles

Remote profiles live at:

```text
~/.config/mdc/profiles.toml
```

To get started:

```bash
mkdir -p ~/.config/mdc
cp profiles.example.toml ~/.config/mdc/profiles.toml
```

Example:

```toml
[[profiles]]
name = "lab-agent"
host = "192.168.1.30"
port = 22
user = "nexus"
path = "/home/nexus"
auth = "agent"
known_hosts_file = "~/.ssh/known_hosts"

[[profiles]]
name = "deploy-key"
host = "files.example.com"
port = 2222
user = "deploy"
path = "/srv/releases"
auth = "key"
identity_file = "~/.ssh/id_ed25519"
known_hosts_file = "~/.ssh/known_hosts"
```

Supported fields:

- `name`
- `host`
- `port`
- `user`
- `path`
- `auth`
- `identity_file`
- `known_hosts_file`

Supported auth modes in Phase 2:

- `agent`
- `key`

Password auth is intentionally not enabled yet.

### Transfer and audit files

Midday Commander currently writes runtime transfer metadata to:

- `~/.config/mdc/audit.log`
- `~/.config/mdc/secrets.json`

`audit.log` is a JSONL audit trail for queued transfer activity.

`secrets.json` is the encrypted fallback secrets store used only as early scaffolding for later secret-provider work. Platform-native stores are still pending.

### Manual connect

The manual connect dialog lets you enter:

- host
- port
- user
- path
- auth mode
- identity file
- known hosts file

It builds the same canonical SFTP URI shape used internally by profiles and bookmarks.

### Raw URI fallback

`Ctrl-G` still accepts raw SFTP URIs when you need to debug or test a connection directly.

Examples:

```text
sftp://nexus@192.168.1.30/home/nexus?auth=agent
sftp://deploy@files.example.com:2222/srv/releases?auth=key&identity_file=~/.ssh/id_ed25519&known_hosts_file=~/.ssh/known_hosts
```

### Remote operations available now

- browse remote directories
- enter child directories and return to parent directories
- create remote directories
- rename remote files and directories
- delete remote files and directories
- bookmark remote directories

### Remote limitations in the current build

- `F5` and `F6` involving SFTP are blocked intentionally
- remote transfers do not use a queue yet
- remote view/edit are not exposed yet
- remote fuzzy find is not exposed yet
- secrets backends are not implemented yet

## Archives

Press `Enter` on any supported archive file to browse its contents as a virtual directory. `Backspace` exits the archive and restores the cursor to the archive file.

Supported formats: `.tar`, `.tar.gz`, `.tar.bz2`, `.tar.xz`, `.zip`, `.7z`, `.rar`, `.gz`, `.bz2`, `.xz`, `.lz4`, `.lz`, `.zst`

## Configuration

UI configuration lives at:

```text
~/.config/mdc/config.toml
```

Remote profile configuration lives at:

```text
~/.config/mdc/profiles.toml
```

Copy the examples to get started:

```bash
mkdir -p ~/.config/mdc
cp config.example.toml ~/.config/mdc/config.toml
cp profiles.example.toml ~/.config/mdc/profiles.toml
```

### Example config

```toml
# Theme (loads from ~/.config/mdc/themes/<name>.toml)
theme = "catppuccin-mocha"

[behavior]
enter_action = "edit"
space_action = "preview"

[keys]
quit           = ["f10", "ctrl+c"]
toggle_panel   = "tab"
copy           = "f5"
move           = "f6"
mkdir          = "f7"
delete         = "f8"
rename         = "shift+f6"
bookmarks      = ["f2", "ctrl+b"]
goto           = "ctrl+g"
remote_connect = ["ctrl+k", "shift+f2"]
fuzzy_find     = ["f9", "ctrl+p"]
theme_picker   = "ctrl+t"
cmd_exec       = "ctrl+r"
```

See [`config.example.toml`](config.example.toml) and [`profiles.example.toml`](profiles.example.toml) for the full references.

## Security Notes

Phase 2 remote access is intentionally conservative:

- host keys are verified strictly against `known_hosts`
- `~/.ssh/known_hosts` is the default unless overridden
- profiles store connection metadata only, not plaintext passwords
- password auth is not enabled yet
- passphrase-protected private keys are expected to be handled through `ssh-agent`

If a host key is missing or mismatched, the connection fails instead of prompting to trust it implicitly.

## Themes

Themes are TOML files stored at `~/.config/mdc/themes/`.

### Live theme picker

Press `Ctrl-T` to open the theme picker overlay. Use `Up` / `Down` to browse themes with live preview. Press `Enter` to apply the selected theme or `Esc` to cancel and revert to the previous theme.

### Installing themes

```bash
mkdir -p ~/.config/mdc/themes
cp themes/*.toml ~/.config/mdc/themes/
```

### Theme format

Themes use a `[palette]` section to define named colors, then reference them throughout:

```toml
name = "My Theme"

[palette]
bg    = "#1e1e2e"
fg    = "#cdd6f4"
blue  = "#89b4fa"
green = "#a6e3a1"

[panel]
border_fg        = "blue"
border_bg        = "bg"
border_active_fg = "fg"
border_active_bg = "bg"

[panel.file]
normal_fg = "fg"
normal_bg = "bg"
dir_fg    = "blue"
dir_bold  = true
exec_fg   = "green"

[statusbar]
fg = "bg"
bg = "blue"

[menubar]
fg            = "bg"
bg            = "blue"
fkey_hint_fg  = "fg"
fkey_hint_bg  = "bg"
fkey_label_fg = "bg"
fkey_label_bg = "blue"
```

Colors can be hex values, ANSI color numbers, or palette references. Missing values fall back to the built-in default theme.

## Contributing

Contributions are welcome. Please open an issue to discuss significant changes before submitting a pull request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes (`git commit -am 'Add my feature'`)
4. Push to the branch (`git push origin feature/my-feature`)
5. Open a pull request

### Development

```bash
git clone https://github.com/kooler/MiddayCommander.git
cd MiddayCommander
make build
make test
```

## License

MIT License. See [LICENSE](LICENSE) for details.
