package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestRecordSlotSuccess_SaveFailure_KeepsClaim(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	job := models.BackupJob{
		ID: "job-save-fail",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "05:00",
		},
	}
	now := time.Date(2026, 7, 7, 5, 0, 0, 0, time.Local)
	if !TryClaimSlot(job, now) {
		t.Fatal("expected claim")
	}
	path, err := scheduleClaimPath(job.ID, slotKey(job, now))
	if err != nil {
		t.Fatal(err)
	}

	statePath, err := scheduleStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{"), 0o444); err != nil {
		t.Fatal(err)
	}

	recordSlotSuccess(job, now)

	if _, err := os.Stat(path); err != nil {
		t.Fatal("claim should remain when durable save fails")
	}
}

func TestRecordSlotSuccess_CorruptJSON_DoesNotOverwriteNeighbor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	neighborID := "neighbor-job"
	neighborKey := "2026-07-07T05:00"
	raw := `{"last_run":{"` + neighborID + `":"` + neighborKey + `"}}`
	corrupt := raw + "NOT_JSON"

	statePath, err := scheduleStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	job := models.BackupJob{
		ID: "other-job",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "06:00",
		},
	}
	now := time.Date(2026, 7, 7, 6, 0, 0, 0, time.Local)
	recordSlotSuccess(job, now)

	got, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != corrupt {
		t.Fatalf("schedule_state overwritten on bad JSON:\nwas: %q\ngot: %q", corrupt, string(got))
	}
}

func TestRecordSlotSuccess_ValidJSON_PreservesNeighborLastRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	neighborID := "neighbor-job"
	neighborKey := "2026-07-07T05:00"
	statePath, err := scheduleStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"last_run":{"`+neighborID+`":"`+neighborKey+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	job := models.BackupJob{
		ID: "other-job",
		Schedule: models.Schedule{
			Enabled: true,
			Type:    "daily",
			Time:    "06:00",
		},
	}
	now := time.Date(2026, 7, 7, 6, 0, 0, 0, time.Local)
	if !TryClaimSlot(job, now) {
		t.Fatal("expected claim")
	}
	recordSlotSuccess(job, now)

	st, err := loadScheduleState()
	if err != nil {
		t.Fatal(err)
	}
	if st.LastRun[neighborID] != neighborKey {
		t.Fatalf("neighbor LastRun lost: %+v", st.LastRun)
	}
	if st.LastRun[job.ID] != slotKey(job, now) {
		t.Fatalf("other job LastRun not saved: %+v", st.LastRun)
	}
}
