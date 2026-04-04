package panel

import (
	"sort"
	"strings"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

type SortMode int

const (
	SortByName SortMode = iota
	SortBySize
	SortByTime
	SortByExtension
)

func SortEntries(entries []midfs.Entry, mode SortMode) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Name == ".." {
			return true
		}
		if b.Name == ".." {
			return false
		}

		aDir := a.IsDir()
		bDir := b.IsDir()
		if aDir != bDir {
			return aDir
		}

		switch mode {
		case SortBySize:
			if a.Size != b.Size {
				return a.Size < b.Size
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortByTime:
			if !a.ModTime.Equal(b.ModTime) {
				return a.ModTime.After(b.ModTime)
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortByExtension:
			aExt := extensionOf(a.Name)
			bExt := extensionOf(b.Name)
			if aExt != bExt {
				return aExt < bExt
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

func extensionOf(name string) string {
	for index := len(name) - 1; index >= 0; index-- {
		if name[index] == '.' {
			return strings.ToLower(name[index:])
		}
	}
	return ""
}
