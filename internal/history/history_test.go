package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

func TestClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	_ = Append(models.JobRunResult{
		JobName:   "test",
		Status:    "ok",
		StartedAt: time.Now(),
	})

	records, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}

	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	records, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("want 0 after clear, got %d", len(records))
	}

	p := filepath.Join(dir, paths.AppFolderName, "history.json")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("history file missing: %v", err)
	}
}
