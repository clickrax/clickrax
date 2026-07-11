//go:build windows

package credential

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestDPAPISecretRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", old) }()

	id := uuid.NewString()
	secret := "pbs-token-secret-value"
	if err := writeDPAPISecret(id, secret); err != nil {
		t.Fatal(err)
	}
	got, err := readDPAPISecret(id)
	if err != nil {
		t.Fatal(err)
	}
	if got != secret {
		t.Fatalf("got %q want %q", got, secret)
	}
	p, _ := secretFilePath(id)
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != id+".dpapi" {
		t.Fatalf("unexpected path %s", p)
	}
}
