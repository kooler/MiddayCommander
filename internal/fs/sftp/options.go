package sftp

import (
	"fmt"
	"path"
	"strings"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/profiles"
)

const (
	QueryAuth           = "auth"
	QueryIdentityFile   = "identity_file"
	QueryKnownHostsFile = "known_hosts_file"
)

// Options describes a normalized SFTP connection target.
type Options struct {
	Host           string
	Port           int
	User           string
	Path           string
	Auth           string
	IdentityFile   string
	KnownHostsFile string
}

// FromProfile builds SFTP options from a remote profile.
func FromProfile(profile profiles.Profile) (Options, error) {
	normalized, err := profiles.Normalize(profile)
	if err != nil {
		return Options{}, err
	}

	return Options{
		Host:           normalized.Host,
		Port:           normalized.Port,
		User:           normalized.User,
		Path:           cleanPath(normalized.Path),
		Auth:           normalized.Auth,
		IdentityFile:   normalized.IdentityFile,
		KnownHostsFile: normalized.KnownHostsFile,
	}, nil
}

// FromURI extracts SFTP options from an fs.URI.
func FromURI(uri midfs.URI) (Options, error) {
	if uri.Scheme != midfs.SchemeSFTP {
		return Options{}, fmt.Errorf("unsupported scheme %q", uri.Scheme)
	}

	normalized, err := profiles.Normalize(profiles.Profile{
		Name:           "sftp",
		Host:           strings.TrimSpace(uri.Host),
		Port:           uri.Port,
		User:           strings.TrimSpace(uri.User),
		Path:           cleanPath(uri.Path),
		Auth:           strings.TrimSpace(uri.QueryValue(QueryAuth)),
		IdentityFile:   strings.TrimSpace(uri.QueryValue(QueryIdentityFile)),
		KnownHostsFile: strings.TrimSpace(uri.QueryValue(QueryKnownHostsFile)),
	})
	if err != nil {
		return Options{}, err
	}

	return Options{
		Host:           normalized.Host,
		Port:           normalized.Port,
		User:           normalized.User,
		Path:           cleanPath(normalized.Path),
		Auth:           normalized.Auth,
		IdentityFile:   normalized.IdentityFile,
		KnownHostsFile: normalized.KnownHostsFile,
	}, nil
}

// URI converts options into a canonical sftp URI.
func (o Options) URI() midfs.URI {
	auth := strings.TrimSpace(o.Auth)
	if auth == "" {
		auth = profiles.AuthAgent
	}

	uri := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   strings.TrimSpace(o.Host),
		Port:   o.Port,
		User:   strings.TrimSpace(o.User),
		Path:   cleanPath(o.Path),
		Query: map[string]string{
			QueryAuth: auth,
		},
	}

	if value := strings.TrimSpace(o.IdentityFile); value != "" {
		uri.Query[QueryIdentityFile] = value
	}
	if value := strings.TrimSpace(o.KnownHostsFile); value != "" {
		uri.Query[QueryKnownHostsFile] = value
	}

	return uri
}

func cleanPath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")))
	if clean == "." {
		return "/"
	}
	return clean
}
