package profiles

const (
	DefaultPort = 22

	AuthAgent = "agent"
	AuthKey   = "key"
)

// Profile describes a named remote endpoint for future SFTP connections.
type Profile struct {
	Name           string `toml:"name"`
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	User           string `toml:"user"`
	Path           string `toml:"path"`
	Auth           string `toml:"auth"`
	IdentityFile   string `toml:"identity_file"`
	KnownHostsFile string `toml:"known_hosts_file"`
}
