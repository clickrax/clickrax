package restore

import (
	"os"
	"path/filepath"
	"testing"

	"pbs-win-backup/internal/pbsbackup"
)

func TestWriteRestoredFilePreservesOriginalOnFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "data.bin")
	original := []byte("original-content-kept")
	if err := os.WriteFile(dest, original, 0o644); err != nil {
		t.Fatal(err)
	}

	badDest := filepath.Join(dir, string([]byte{0}), "missing", "file.bin")
	err := pbsbackup.WriteRestoredFile(badDest, []byte("new"))
	if err == nil {
		t.Fatal("expected error for invalid destination path")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("original corrupted: got %q", got)
	}
}

func TestWriteRestoredFileAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")
	payload := []byte("restored-payload")
	if err := pbsbackup.WriteRestoredFile(dest, payload); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(dest + ".restoring"); !os.IsNotExist(err) {
		t.Fatal("temp file should be removed")
	}
}
