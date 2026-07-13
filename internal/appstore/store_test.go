package appstore

import (
	"testing"

	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/models"
)

func TestStore_Update_ReloadsFromDiskBeforeSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	diskCfg := config.DefaultConfig()
	diskCfg.Jobs = []models.BackupJob{{ID: "neighbor", Name: "keep-me"}}
	if err := config.Save(diskCfg); err != nil {
		t.Fatal(err)
	}

	stale := config.DefaultConfig()
	stale.Jobs = []models.BackupJob{{ID: "stale-only", Name: "gone"}}
	s := New(stale)

	if err := s.Update(func(cfg *models.Config) error {
		cfg.Jobs = append(cfg.Jobs, models.BackupJob{ID: "added", Name: "new"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2 (neighbor + added)", len(loaded.Jobs))
	}
	byID := map[string]string{}
	for _, j := range loaded.Jobs {
		byID[j.ID] = j.Name
	}
	if byID["neighbor"] != "keep-me" {
		t.Fatalf("neighbor job lost: %+v", byID)
	}
	if byID["added"] != "new" {
		t.Fatalf("added job missing: %+v", byID)
	}
	if _, ok := byID["stale-only"]; ok {
		t.Fatal("stale in-memory job should not overwrite disk")
	}
}
