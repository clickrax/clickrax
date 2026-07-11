package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigHasUserData(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`{"version":1,"jobs":[],"settings":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if configHasUserData(empty) {
		t.Fatal("empty config should not count as user data")
	}

	full := filepath.Join(dir, "full.json")
	if err := os.WriteFile(full, []byte(`{"destinations":[{"id":"x"}],"jobs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !configHasUserData(full) {
		t.Fatal("destinations should count as user data")
	}
}

func TestMigrateLegacyIfNeeded(t *testing.T) {
	root := t.TempDir()
	programData := filepath.Join(root, "ProgramData")
	legacyDir := filepath.Join(programData, legacyAppFolderName)
	newDir := filepath.Join(programData, AppFolderName)

	if err := os.MkdirAll(filepath.Join(legacyDir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyCfg := `{"version":1,"destinations":[{"id":"srv1","name":"pbs"}],"jobs":[{"id":"j1"}]}`
	if err := os.WriteFile(filepath.Join(legacyDir, "config.json"), []byte(legacyCfg), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "secrets", "x.dpapi"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "config.json"), []byte(`{"version":1,"jobs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyIfNeeded(programData); err != nil {
		t.Fatal(err)
	}
	if !configHasUserData(filepath.Join(newDir, "config.json")) {
		t.Fatal("migrated config should contain legacy destinations")
	}
	if _, err := os.Stat(filepath.Join(newDir, "secrets", "x.dpapi")); err != nil {
		t.Fatalf("secrets should be migrated: %v", err)
	}
}

func TestDataDirPrefersLegacyWhenClickRAXEmpty(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramData", root)

	legacyDir := filepath.Join(root, legacyAppFolderName)
	newDir := filepath.Join(root, AppFolderName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "config.json"), []byte(`{"destinations":[{"id":"a"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "config.json"), []byte(`{"jobs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != newDir {
		t.Fatalf("DataDir=%q want migrated ClickRAX %q", got, newDir)
	}
	if !configHasUserData(filepath.Join(newDir, "config.json")) {
		t.Fatal("expected migrated config in ClickRAX")
	}
}
