// Package quickview implements an embedded, read-only file preview that can
// replace the inactive panel. It mirrors the panel's bordered box so the two
// sit side-by-side seamlessly. Content follows the active panel's cursor and
// is a bounded head of the file. A local file is read inline; a remote one is
// read by a command, because a network round trip must not block the event
// loop.
package quickview

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
	"github.com/kooler/MiddayCommander/internal/vfs"
)

// maxPreviewBytes is how much of a file we read for the preview. We only ever
// load the head so following the cursor across large files stays cheap.
const maxPreviewBytes = 256 * 1024

// nextSeq numbers preview instances so a read can be routed back to the
// preview that asked for it.
var nextSeq atomic.Uint64

type contentKind int

const (
	kindText contentKind = iota
	kindBinary
	kindDir
	kindEmpty
	kindError
	kindUnavailable
	kindLoading
)

// Model is the file preview sub-model.
type Model struct {
	// id travels with a remote read so the app can route the result back here.
	id        uint64
	path      string
	name      string
	lines     []string // text content lines (kindText only)
	offset    int      // scroll position into lines
	width     int      // total box width (including borders)
	height    int      // rows available for content (excluding header+footer)
	focused   bool     // true when keystrokes scroll the preview
	kind      contentKind
	truncated bool // file was longer than what we loaded
	info      fs.FileInfo
	errMsg    string
}

// New creates an empty preview.
func New() Model { return Model{id: nextSeq.Add(1)} }

// SetSize sets the box dimensions. height is the content row count (matching the
// panel's list height) so the bordered box aligns with the sibling panel.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampOffset()
}

// SetFocused marks whether the preview currently receives scroll keys.
func (m *Model) SetFocused(f bool) { m.focused = f }

// Focused reports whether the preview currently receives scroll keys.
func (m Model) Focused() bool { return m.focused }

// Path returns the file currently previewed.
func (m Model) Path() string { return m.path }

// FileLoadedMsg carries a preview read that happened off the event loop.
type FileLoadedMsg struct {
	// ID is the preview that asked for the read.
	ID        uint64
	Path      string
	Data      []byte
	Truncated bool
	Err       error
}

// SetFile points the preview at an entry, resetting scroll. available is false
// when the entry cannot be read at all, as inside an archive. A returned
// command must be run: the content arrives as a FileLoadedMsg.
func (m *Model) SetFile(ref vfs.FileRef, info fs.FileInfo, isDir, available bool) tea.Cmd {
	m.path = ref.Path
	m.name = ref.Base()
	m.info = info
	m.offset = 0
	m.truncated = false
	m.errMsg = ""
	m.lines = nil

	switch {
	case !available:
		m.kind = kindUnavailable
	case isDir:
		m.kind = kindDir
	case ref.IsLocal():
		data, truncated, err := readHead(ref)
		m.applyContent(data, truncated, err)
	default:
		m.kind = kindLoading
		return loadFileCmd(m.id, ref)
	}
	return nil
}

// HandleFileLoaded applies a completed read, ignoring one meant for another
// preview or one the cursor moved past.
func (m *Model) HandleFileLoaded(msg FileLoadedMsg) {
	if msg.ID != 0 && msg.ID != m.id {
		return // belongs to another preview
	}
	if msg.Path != m.path || m.kind != kindLoading {
		return
	}
	m.applyContent(msg.Data, msg.Truncated, msg.Err)
}

func loadFileCmd(id uint64, ref vfs.FileRef) tea.Cmd {
	return func() tea.Msg {
		data, truncated, err := readHead(ref)
		return FileLoadedMsg{ID: id, Path: ref.Path, Data: data, Truncated: truncated, Err: err}
	}
}

// readHead reads the bounded head of a file, reporting whether more remained.
func readHead(ref vfs.FileRef) ([]byte, bool, error) {
	f, err := ref.FS.Open(ref.Path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	// Read one byte past the cap so we can tell whether the file was truncated.
	buf, err := io.ReadAll(io.LimitReader(f, maxPreviewBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(buf) > maxPreviewBytes {
		return buf[:maxPreviewBytes], true, nil
	}
	return buf, false, nil
}

func (m *Model) applyContent(buf []byte, truncated bool, err error) {
	m.truncated = truncated
	if err != nil {
		m.kind = kindError
		m.errMsg = err.Error()
		return
	}
	if len(buf) == 0 {
		m.kind = kindEmpty
		return
	}
	if isBinary(buf) {
		m.kind = kindBinary
		return
	}
	m.kind = kindText
	m.lines = splitLines(buf)
}

// Update handles scroll keys; only meaningful while focused.
func (m *Model) Update(msg tea.KeyMsg) {
	visible := len(m.lines)
	if m.kind != kindText {
		visible = 0
	}
	maxOff := max(0, visible-m.height)
	switch msg.String() {
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.offset -= m.height
	case "pgdown":
		m.offset += m.height
	case "home", "g":
		m.offset = 0
	case "end", "G":
		m.offset = maxOff
	}
	m.clampOffset()
}

func (m *Model) clampOffset() {
	visible := len(m.lines)
	if m.kind != kindText {
		visible = 0
	}
	// quickview scrolls offset directly (no cursor); feed offset as cursor so
	// ClampScroll preserves it while capping at max(0, visible-height).
	m.offset = overlay.ClampScroll(m.offset, m.offset, m.height, visible)
}

// View renders the preview as a bordered box, matching the panel layout.
func (m Model) View(th theme.Theme, focused bool) string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	borderStyle := th.PanelBorder
	headerStyle := th.PanelHeader
	if focused {
		borderStyle = th.PanelBorderActive
		headerStyle = th.PanelHeaderActive
	}

	innerWidth := m.width - 2

	// Header: filename + preview tag.
	header := m.name + " [preview]"
	headerLine := borderStyle.Render("┌") +
		headerStyle.Render(" "+overlay.PadOrTruncDots(header, innerWidth-2)+" ") +
		borderStyle.Render("┐")

	// Body.
	content := m.contentLines(innerWidth, th.FileNormal)
	var rows []string
	end := m.offset + m.height
	if end > len(content) {
		end = len(content)
	}
	for i := m.offset; i < end; i++ {
		rows = append(rows, borderStyle.Render("│")+content[i]+borderStyle.Render("│"))
	}
	emptyRow := th.FileNormal.Render(strings.Repeat(" ", innerWidth))
	for len(rows) < m.height {
		rows = append(rows, borderStyle.Render("│")+emptyRow+borderStyle.Render("│"))
	}

	// Footer.
	footerLine := borderStyle.Render("└") +
		headerStyle.Render(overlay.PadOrTruncDots(m.footerText(), innerWidth)) +
		borderStyle.Render("┘")

	parts := []string{headerLine}
	parts = append(parts, rows...)
	parts = append(parts, footerLine)
	return strings.Join(parts, "\n")
}

// contentLines returns the styled, width-clamped body lines for the current kind.
func (m Model) contentLines(width int, normal lipgloss.Style) []string {
	render := func(ss []string) []string {
		out := make([]string, len(ss))
		for i, s := range ss {
			out[i] = normal.Render(overlay.PadOrTruncDots(s, width))
		}
		return out
	}

	switch m.kind {
	case kindText:
		return render(m.lines)
	case kindBinary:
		return render(m.centered(width, "⟨ binary file ⟩", m.name, m.sizeStr()))
	case kindEmpty:
		return render(m.centered(width, "⟨ empty file ⟩", m.name))
	case kindDir:
		return render(m.centered(width, "⟨ directory ⟩", m.name))
	case kindUnavailable:
		return render(m.centered(width, "⟨ preview unavailable ⟩", m.name))
	case kindError:
		return render(m.centered(width, "⟨ cannot preview ⟩", m.errMsg))
	case kindLoading:
		return render(m.centered(width, "⟨ loading… ⟩", m.name))
	default:
		return nil
	}
}

// centered builds a vertically/horizontally centered message block.
func (m Model) centered(width int, msgs ...string) []string {
	lines := make([]string, 0, m.height)
	top := (m.height - len(msgs)) / 2
	for i := 0; i < top; i++ {
		lines = append(lines, "")
	}
	for _, s := range msgs {
		if pad := (width - ansi.StringWidth(s)) / 2; pad > 0 {
			s = strings.Repeat(" ", pad) + s
		}
		lines = append(lines, s)
	}
	return lines
}

func (m Model) footerText() string {
	switch m.kind {
	case kindText:
		maxOff := 0
		if maxOff = len(m.lines) - m.height; maxOff < 0 {
			maxOff = 0
		}
		pct := 100
		if maxOff > 0 {
			pct = m.offset * 100 / maxOff
		}
		s := fmt.Sprintf(" %d%% ", pct)
		if m.truncated {
			s = " head " + s
		}
		return s
	case kindUnavailable, kindDir, kindEmpty, kindBinary, kindError:
		return " preview "
	default:
		return ""
	}
}

func (m Model) sizeStr() string {
	if m.info == nil {
		return ""
	}
	return formatSize(m.info.Size())
}

// --- helpers ---

func isBinary(b []byte) bool {
	const sample = 8000
	if len(b) > sample {
		b = b[:sample]
	}
	nonPrintable := 0
	for _, c := range b {
		if c == 0 {
			return true
		}
		if c < 0x09 || (c > 0x0d && c < 0x20) {
			nonPrintable++
		}
	}
	return len(b) > 0 && nonPrintable*100/len(b) > 30
}

func splitLines(b []byte) []string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Split(s, "\n")
}

func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
