package backupqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/history"
)

func TestRecordDeadLetter_PanicRecordsHistoryError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{
		JobID:      "panic-job",
		Trigger:    "manual",
		JobName:    "Panic",
		EnqueuedAt: time.Now(),
	}
	RecordDeadLetter(item, fmt.Errorf("%w: simulated panic", ErrQueuePanic))

	dead, err := loadDeadLetters()
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 1 || dead[0].Item.JobID != item.JobID {
		t.Fatalf("dead-letter = %+v", dead)
	}

	records, err := history.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("history records = %d, want 1", len(records))
	}
	if records[0].Status != "error" || records[0].JobID != item.JobID {
		t.Fatalf("history record = %+v", records[0])
	}
}
