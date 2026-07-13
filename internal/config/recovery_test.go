//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverFromQuarantinePicksRichest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	root := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"version":1,"destinations":[],"jobs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rich := `{"version":1,"destinations":[{"id":"a","type":"pbs","name":"pbs1","url":"https://pbs.example.local:8007"}],"jobs":[{"id":"j1"},{"id":"j2"}]}`
	if err := os.WriteFile(filepath.Join(root, "config.json.corrupt-20260101-120000"), []byte(rich), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := RecoverFromQuarantine(cfgPath)
	if err != nil || !ok {
		t.Fatalf("recover: ok=%v err=%v", ok, err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Destinations) != 1 || len(cfg.Jobs) != 2 {
		t.Fatalf("got dest=%d jobs=%d", len(cfg.Destinations), len(cfg.Jobs))
	}
}
