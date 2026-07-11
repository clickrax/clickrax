package pbsbackup

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestListInterruptedCheckpoints(t *testing.T) {
	jobID := "test-interrupted-" + time.Now().Format("150405")
	t.Cleanup(func() { _ = ClearCheckpoint(jobID) })

	if err := SaveCheckpoint(Checkpoint{
		JobID:     jobID,
		JobName:   "Test",
		Phase:     string(models.PhaseTransfer),
		Percent:   42,
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	list, err := ListInterruptedCheckpoints()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cp := range list {
		if cp.JobID == jobID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("interrupted checkpoint not listed")
	}

	if err := ClearCheckpoint(jobID); err != nil {
		t.Fatal(err)
	}
	list, err = ListInterruptedCheckpoints()
	if err != nil {
		t.Fatal(err)
	}
	for _, cp := range list {
		if cp.JobID == jobID {
			t.Fatal("cleared checkpoint still listed")
		}
	}
}

func TestListActiveCheckpoints_excludesCancelled(t *testing.T) {
	jobID := "test-active-cancel-" + time.Now().Format("150405")
	t.Cleanup(func() { _ = ClearCheckpoint(jobID) })

	if err := SaveCheckpoint(Checkpoint{
		JobID:     jobID,
		JobName:   "Test",
		Phase:     string(models.PhaseCancelled),
		Percent:   42,
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	list, err := ListActiveCheckpoints(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, cp := range list {
		if cp.JobID == jobID {
			t.Fatal("cancelled checkpoint must not be listed as active")
		}
	}

	interrupted, err := ListInterruptedCheckpoints()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cp := range interrupted {
		if cp.JobID == jobID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cancelled checkpoint should appear in interrupted list")
	}
}
