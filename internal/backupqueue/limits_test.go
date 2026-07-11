package backupqueue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnqueueQueueFull(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	origMax := MaxQueueItems
	// Override via small cap for test by filling manually.
	items := make([]Item, MaxQueueItems)
	for i := range items {
		items[i] = Item{JobID: "job", Trigger: "manual", EnqueuedAt: time.Now()}
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(progData, "backup_queue.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = origMax

	if _, err := Enqueue(Item{JobID: "overflow", Trigger: "manual"}); err == nil {
		t.Fatal("expected queue full error")
	}
}

func TestAppendDeadLetterTrims(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < MaxDeadLetterRecords+5; i++ {
		if err := appendDeadLetter(deadLetterRecord{
			Item:   Item{JobID: "job", Trigger: "manual"},
			LostAt: time.Now(),
			Reason: "test",
		}); err != nil {
			t.Fatal(err)
		}
	}
	records, err := loadDeadLetters()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != MaxDeadLetterRecords {
		t.Fatalf("got %d dead letters, want %d", len(records), MaxDeadLetterRecords)
	}
}
