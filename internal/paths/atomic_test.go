package paths

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_PreservesOriginalOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	original := []byte("original content")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	oldRename := renameFile
	defer func() { renameFile = oldRename }()

	renameFile = func(_, _ string) error {
		return errors.New("rename failed")
	}

	err := AtomicWrite(target, []byte("new content"), 0o644)
	if err == nil {
		t.Fatal("expected error from AtomicWrite")
	}

	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("original file should still exist: %v", readErr)
	}
	if string(data) != string(original) {
		t.Fatalf("original content preserved: got %q, want %q", data, original)
	}
}
