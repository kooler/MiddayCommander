package quickview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func statOf(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info
}

func TestSetFileClassification(t *testing.T) {
	dir := t.TempDir()

	textPath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(textPath, []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 'a', 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(dir, "empty")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		path      string
		isDir     bool
		available bool
		want      contentKind
	}{
		{"text", textPath, false, true, kindText},
		{"binary", binPath, false, true, kindBinary},
		{"empty", emptyPath, false, true, kindEmpty},
		{"dir", dir, true, true, kindDir},
		{"unavailable", "some/archive/path", false, false, kindUnavailable},
		{"missing", filepath.Join(dir, "nope"), false, true, kindError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m Model
			var info os.FileInfo
			if tc.available && !strings.Contains(tc.path, "nope") {
				if fi, err := os.Stat(tc.path); err == nil {
					info = fi
				}
			}
			m.SetFile(tc.path, info, tc.isDir, tc.available)
			if m.kind != tc.want {
				t.Errorf("kind = %d, want %d", m.kind, tc.want)
			}
		})
	}
}

func TestTextLinesLoaded(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	var m Model
	m.SetFile(p, statOf(t, p), false, true)
	if got := len(m.lines); got != 3 {
		t.Fatalf("lines = %d, want 3", got)
	}
}

func TestTruncationFlag(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	big := strings.Repeat("x", maxPreviewBytes+100)
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	var m Model
	m.SetFile(p, statOf(t, p), false, true)
	if !m.truncated {
		t.Error("expected truncated = true for oversized file")
	}
}
