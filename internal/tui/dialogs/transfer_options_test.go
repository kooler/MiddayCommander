package dialogs

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/transfer"
)

func TestTransferOptionsSubmitDefaults(t *testing.T) {
	model := NewTransferOptions(transfer.OperationCopy, 2, midfs.NewFileURI("/tmp/dest"), 80, 24)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Update(enter) did not return submit command")
	}

	msg := cmd()
	submit, ok := msg.(TransferOptionsSubmitMsg)
	if !ok {
		t.Fatalf("submit msg = %T, want dialogs.TransferOptionsSubmitMsg", msg)
	}
	if submit.Operation != transfer.OperationCopy {
		t.Fatalf("submit.Operation = %q, want %q", submit.Operation, transfer.OperationCopy)
	}
	if submit.Conflict != transfer.ConflictOverwrite {
		t.Fatalf("submit.Conflict = %q, want overwrite", submit.Conflict)
	}
	if submit.Verify != transfer.VerifySize {
		t.Fatalf("submit.Verify = %q, want size", submit.Verify)
	}
	if submit.Retries != 1 {
		t.Fatalf("submit.Retries = %d, want 1", submit.Retries)
	}
	if updated.retries != 1 {
		t.Fatalf("updated.retries = %d, want 1", updated.retries)
	}
}

func TestTransferOptionsCyclesValues(t *testing.T) {
	model := NewTransferOptions(transfer.OperationMove, 1, midfs.NewFileURI("/tmp/dest"), 80, 24)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	if model.conflict != transfer.ConflictSkip {
		t.Fatalf("conflict after right = %q, want skip", model.conflict)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	if model.verify != transfer.VerifySHA256 {
		t.Fatalf("verify after right = %q, want sha256", model.verify)
	}

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if model.retries != 3 {
		t.Fatalf("retries after digit = %d, want 3", model.retries)
	}
}
