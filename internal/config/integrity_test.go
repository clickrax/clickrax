//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigIntegrityRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	data := []byte(`{"version":1,"jobs":[]}`)
	if _, err := configHMACKey(); err != nil {
		t.Fatal(err)
	}
	sig, err := signConfigBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(progData, "config.json")
	sigPath := configSigPath(cfgPath)
	if err := os.WriteFile(sigPath, []byte(sig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyConfigBytes(data, sigPath); err != nil {
		t.Fatal(err)
	}
	tampered := []byte(`{"version":1,"jobs":[{"id":"x"}]}`)
	if err := verifyConfigBytes(tampered, sigPath); err == nil {
		t.Fatal("expected tampered config to fail verification")
	}
	// Valid JSON with stale HMAC is auto-healed on load (ensureConfigSignature re-signs).
	stale := []byte(`{"version":1,"jobs":[]}`)
	if err := ensureConfigSignature(cfgPath, stale); err != nil {
		t.Fatalf("auto-heal stale hmac: %v", err)
	}
}
