package fs

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Scheme string

const (
	SchemeFile    Scheme = "file"
	SchemeArchive Scheme = "archive"
	SchemeSFTP    Scheme = "sftp"
)

type URI struct {
	Scheme Scheme
	Host   string
	Port   int
	User   string
	Path   string
	Query  map[string]string
}

type EntryType string

const (
	EntryFile    EntryType = "file"
	EntryDir     EntryType = "dir"
	EntrySymlink EntryType = "symlink"
	EntryArchive EntryType = "archive"
	EntryUnknown EntryType = "unknown"
)

type Entry struct {
	Name      string
	Path      string
	URI       URI
	Type      EntryType
	Size      int64
	Mode      os.FileMode
	ModTime   time.Time
	Target    string
	Readable  bool
	Writable  bool
	Hidden    bool
	IsArchive bool
}

func (e Entry) IsDir() bool {
	return e.Type == EntryDir
}

func (e Entry) IsSymlink() bool {
	return e.Type == EntrySymlink
}

func (e Entry) DisplayName() string {
	if e.Name != "" {
		return e.Name
	}
	return Base(e.URI)
}

type OpenReadOptions struct {
	Offset int64
}

type OpenWriteOptions struct {
	Offset        int64
	Atomic        bool
	Overwrite     bool
	TempExtension string
	Perm          os.FileMode
}

type FileSystem interface {
	ID() string
	Scheme() Scheme
	Capabilities() uint64

	List(ctx context.Context, dir URI) ([]Entry, error)
	Stat(ctx context.Context, uri URI) (Entry, error)

	Mkdir(ctx context.Context, uri URI, perm os.FileMode) error
	Rename(ctx context.Context, from URI, to URI) error
	Remove(ctx context.Context, uri URI, recursive bool) error

	OpenReader(ctx context.Context, uri URI, opts OpenReadOptions) (io.ReadCloser, error)
	OpenWriter(ctx context.Context, uri URI, opts OpenWriteOptions) (io.WriteCloser, error)

	Join(base URI, elems ...string) URI
	Parent(uri URI) URI
	Clean(uri URI) URI

	Close() error
}

func NewFileURI(path string) URI {
	return URI{
		Scheme: SchemeFile,
		Path:   filepath.Clean(path),
	}
}

func NewArchiveURI(archivePath, entry string) URI {
	uri := URI{
		Scheme: SchemeArchive,
		Path:   filepath.Clean(archivePath),
		Query:  map[string]string{},
	}
	uri.Query["entry"] = cleanArchiveEntry(entry)
	return uri
}

func ParseURI(raw string) (URI, error) {
	if raw == "" {
		return URI{}, fmt.Errorf("empty uri")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return URI{}, err
	}
	if parsed.Scheme == "" {
		return URI{}, fmt.Errorf("missing scheme in uri %q", raw)
	}

	uri := URI{
		Scheme: Scheme(parsed.Scheme),
		Host:   parsed.Hostname(),
		User:   parsed.User.Username(),
		Query:  map[string]string{},
	}
	if port := parsed.Port(); port != "" {
		uri.Port, err = strconv.Atoi(port)
		if err != nil {
			return URI{}, fmt.Errorf("parse port %q: %w", port, err)
		}
	}

	switch uri.Scheme {
	case SchemeFile, SchemeArchive:
		uri.Path = fromLocalURIPath(parsed.Path)
	case SchemeSFTP:
		uri.Path = cleanSlashPath(parsed.Path)
	default:
		return URI{}, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}

	for key, values := range parsed.Query() {
		if len(values) == 0 {
			continue
		}
		uri.Query[key] = values[len(values)-1]
	}
	if uri.Scheme == SchemeArchive {
		uri.Query["entry"] = cleanArchiveEntry(uri.Query["entry"])
	}
	if uri.Scheme != SchemeArchive && len(uri.Query) == 0 {
		uri.Query = nil
	}

	return uri, nil
}

func MustParseURI(raw string) URI {
	uri, err := ParseURI(raw)
	if err != nil {
		panic(err)
	}
	return uri
}

func (u URI) String() string {
	queryKeys := make([]string, 0, len(u.Query))
	for key := range u.Query {
		queryKeys = append(queryKeys, key)
	}
	sort.Strings(queryKeys)

	values := url.Values{}
	for _, key := range queryKeys {
		values.Set(key, u.Query[key])
	}

	result := url.URL{
		Scheme: string(u.Scheme),
	}

	switch u.Scheme {
	case SchemeFile, SchemeArchive:
		result.Path = toLocalURIPath(u.Path)
	case SchemeSFTP:
		result.Path = cleanSlashPath(u.Path)
		if u.User != "" {
			result.User = url.User(u.User)
		}
		if u.Port > 0 {
			result.Host = fmt.Sprintf("%s:%d", u.Host, u.Port)
		} else {
			result.Host = u.Host
		}
	default:
		result.Path = u.Path
	}

	if len(values) > 0 {
		result.RawQuery = values.Encode()
	}

	return result.String()
}

func (u URI) QueryValue(key string) string {
	if u.Query == nil {
		return ""
	}
	return u.Query[key]
}

func (u URI) WithQueryValue(key, value string) URI {
	copyURI := u.Clone()
	if copyURI.Query == nil {
		copyURI.Query = map[string]string{}
	}
	copyURI.Query[key] = value
	return copyURI
}

func (u URI) Clone() URI {
	clone := u
	if len(u.Query) == 0 {
		return clone
	}
	clone.Query = make(map[string]string, len(u.Query))
	for key, value := range u.Query {
		clone.Query[key] = value
	}
	return clone
}

func (u URI) Display() string {
	switch u.Scheme {
	case SchemeArchive:
		archiveName := filepath.Base(u.Path)
		entry := cleanArchiveEntry(u.QueryValue("entry"))
		if entry == "." {
			return archiveName + "://"
		}
		return archiveName + "://" + entry
	case SchemeFile:
		return u.Path
	case SchemeSFTP:
		host := u.Host
		if u.Port > 0 {
			host = fmt.Sprintf("%s:%d", host, u.Port)
		}
		if u.User != "" {
			host = u.User + "@" + host
		}
		if host == "" {
			return u.Path
		}
		return host + ":" + u.Path
	default:
		return u.String()
	}
}

func (u URI) IsZero() bool {
	return u.Scheme == "" && u.Path == "" && u.Host == "" && u.Port == 0 && u.User == "" && len(u.Query) == 0
}

func Base(uri URI) string {
	switch uri.Scheme {
	case SchemeArchive:
		entry := cleanArchiveEntry(uri.QueryValue("entry"))
		if entry == "." {
			return filepath.Base(uri.Path)
		}
		return path.Base(entry)
	case SchemeFile:
		return filepath.Base(uri.Path)
	case SchemeSFTP:
		return path.Base(cleanSlashPath(uri.Path))
	default:
		return filepath.Base(uri.Path)
	}
}

func IsRootFilePath(path string) bool {
	clean := filepath.Clean(path)
	if clean == string(filepath.Separator) {
		return true
	}
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return false
	}
	if clean == volume {
		return true
	}
	return clean == volume+string(filepath.Separator)
}

func cleanSlashPath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")))
	if clean == "." {
		return "/"
	}
	return clean
}

func cleanArchiveEntry(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(strings.ReplaceAll(raw, "\\", "/"), "/")
	if raw == "" || raw == "." {
		return "."
	}
	clean := path.Clean(raw)
	if clean == "." {
		return "."
	}
	return strings.TrimPrefix(clean, "/")
}

func toLocalURIPath(localPath string) string {
	if localPath == "" {
		return "/"
	}
	pathValue := filepath.ToSlash(filepath.Clean(localPath))
	if !strings.HasPrefix(pathValue, "/") {
		pathValue = "/" + pathValue
	}
	return pathValue
}

func fromLocalURIPath(uriPath string) string {
	if uriPath == "" {
		return ""
	}
	if runtime.GOOS == "windows" && len(uriPath) >= 3 && uriPath[0] == '/' && uriPath[2] == ':' {
		uriPath = uriPath[1:]
	}
	return filepath.Clean(filepath.FromSlash(uriPath))
}
