package panel

import (
	"fmt"
	"time"
)

// FormatSize returns a human-readable file size.
func FormatSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(size)/(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%d", size)
	}
}

// FormatTime returns a formatted modification time.
func FormatTime(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() {
		return t.Format("Jan 02 15:04")
	}
	return t.Format("Jan 02  2006")
}