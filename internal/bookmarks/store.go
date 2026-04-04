package bookmarks

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kooler/MiddayCommander/internal/config"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

type Bookmark struct {
	URI      string    `json:"uri,omitempty"`
	Name     string    `json:"name,omitempty"`
	Count    int       `json:"count"`
	LastUsed time.Time `json:"last_used"`
}

func (b Bookmark) ParsedURI() (midfs.URI, error) {
	return midfs.ParseURI(b.URI)
}

func (b Bookmark) DisplayPath() string {
	uri, err := b.ParsedURI()
	if err != nil {
		return b.URI
	}
	return uri.Display()
}

type Store struct {
	Bookmarks []Bookmark
	path      string
}

type diskStore struct {
	Bookmarks []diskBookmark `json:"bookmarks"`
}

type diskBookmark struct {
	URI      string    `json:"uri,omitempty"`
	Path     string    `json:"path,omitempty"`
	Name     string    `json:"name,omitempty"`
	Count    int       `json:"count"`
	LastUsed time.Time `json:"last_used"`
}

func LoadStore() *Store {
	store := &Store{path: storePath()}

	data, err := os.ReadFile(store.path)
	if err != nil {
		return store
	}

	var persisted diskStore
	if err := json.Unmarshal(data, &persisted); err != nil {
		return store
	}

	store.Bookmarks = make([]Bookmark, 0, len(persisted.Bookmarks))
	for _, bookmark := range persisted.Bookmarks {
		uri := bookmark.URI
		if uri == "" && bookmark.Path != "" {
			uri = midfs.NewFileURI(bookmark.Path).String()
		}
		if uri == "" {
			continue
		}
		store.Bookmarks = append(store.Bookmarks, Bookmark{
			URI:      uri,
			Name:     bookmark.Name,
			Count:    bookmark.Count,
			LastUsed: bookmark.LastUsed,
		})
	}

	return store
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	persisted := diskStore{
		Bookmarks: make([]diskBookmark, 0, len(s.Bookmarks)),
	}
	for _, bookmark := range s.Bookmarks {
		persisted.Bookmarks = append(persisted.Bookmarks, diskBookmark{
			URI:      bookmark.URI,
			Name:     bookmark.Name,
			Count:    bookmark.Count,
			LastUsed: bookmark.LastUsed,
		})
	}

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Add(uri midfs.URI, name string) {
	key := uri.String()
	for index, bookmark := range s.Bookmarks {
		if bookmark.URI != key {
			continue
		}
		s.Bookmarks[index].Count++
		s.Bookmarks[index].LastUsed = time.Now()
		if name != "" {
			s.Bookmarks[index].Name = name
		}
		return
	}

	s.Bookmarks = append(s.Bookmarks, Bookmark{
		URI:      key,
		Name:     name,
		Count:    1,
		LastUsed: time.Now(),
	})
}

func (s *Store) Remove(uri midfs.URI) {
	key := uri.String()
	for index, bookmark := range s.Bookmarks {
		if bookmark.URI != key {
			continue
		}
		s.Bookmarks = append(s.Bookmarks[:index], s.Bookmarks[index+1:]...)
		return
	}
}

func (s *Store) Touch(uri midfs.URI) {
	key := uri.String()
	for index, bookmark := range s.Bookmarks {
		if bookmark.URI != key {
			continue
		}
		s.Bookmarks[index].Count++
		s.Bookmarks[index].LastUsed = time.Now()
		return
	}
}

func (s *Store) Sorted() []Bookmark {
	result := make([]Bookmark, len(s.Bookmarks))
	copy(result, s.Bookmarks)

	now := time.Now()
	sort.Slice(result, func(i, j int) bool {
		return frecency(result[i], now) > frecency(result[j], now)
	})
	return result
}

func (s *Store) Has(uri midfs.URI) bool {
	key := uri.String()
	for _, bookmark := range s.Bookmarks {
		if bookmark.URI == key {
			return true
		}
	}
	return false
}

func frecency(bookmark Bookmark, now time.Time) float64 {
	hoursSince := now.Sub(bookmark.LastUsed).Hours()
	recency := math.Max(0, 100-hoursSince)
	return float64(bookmark.Count)*10 + recency
}

func storePath() string {
	path := config.BookmarksPath()
	if path == "" {
		return "bookmarks.json"
	}
	return path
}
