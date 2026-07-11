//go:build windows

package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"pbs-win-backup/internal/paths"
)

func TestAtomicWriteOverReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "chunks.json")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Simulate service-owned file: Users read-only.
	_ = paths.GrantUsersModify(dir)
	if err := paths.AtomicWrite(target, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("got %q", data)
	}
}
