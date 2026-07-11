package config

import (
	"testing"

	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/models"
)

func TestNormalizeSettings_NetworkRetries(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, 5},
		{-1, 5},
		{3, 3},
		{10, 10},
		{99, MaxNetworkRetries},
	}
	for _, tc := range tests {
		s := models.AppSettings{NetworkRetries: tc.in, LogLevel: "info"}
		NormalizeSettings(&s)
		if s.NetworkRetries != tc.want {
			t.Fatalf("NetworkRetries %d -> %d, want %d", tc.in, s.NetworkRetries, tc.want)
		}
	}
}

func TestEffectiveNetworkRetries(t *testing.T) {
	if got := EffectiveNetworkRetries(0); got != 3 {
		t.Fatalf("zero -> %d, want 3", got)
	}
	if got := EffectiveNetworkRetries(99); got != MaxNetworkRetries {
		t.Fatalf("99 -> %d, want %d", got, MaxNetworkRetries)
	}
}

func TestNormalizeSettings_LogLevel(t *testing.T) {
	eventlog.SetLevel("info")
	NormalizeSettings(&models.AppSettings{LogLevel: "error", NetworkRetries: 3})
	eventlog.Info("should be suppressed")
	eventlog.Error("should log")
}

func TestNormalizeConfig_ServersMigration(t *testing.T) {
	cfg := &models.Config{
		Servers: []models.PBSServer{
			{ID: "srv-1", URL: "https://pbs.example.com:8007", Datastore: "backup"},
		},
		Jobs: []models.BackupJob{
			{ID: "job-1", ServerID: "srv-1", DestinationID: ""},
		},
	}
	migrated := normalizeConfig(cfg)
	if !migrated {
		t.Fatal("expected migration")
	}
	if len(cfg.Destinations) != 1 || cfg.Destinations[0].ID != "srv-1" {
		t.Fatalf("destinations: %+v", cfg.Destinations)
	}
	if cfg.Jobs[0].DestinationID != "srv-1" {
		t.Fatalf("job destination: %q", cfg.Jobs[0].DestinationID)
	}
	if len(cfg.Servers) != 0 {
		t.Fatalf("servers not cleared: %+v", cfg.Servers)
	}
}
