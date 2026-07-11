//go:build windows

package credential

import (
	"errors"
	"os"
	"testing"

	"github.com/danieljoos/wincred"
	"github.com/google/uuid"
)

func TestSetSecret_WincredFails_NoPersistedSecret(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", old) }()

	id := uuid.NewString()
	secret := "should-not-persist"

	oldWrite := writeGenericCredential
	t.Cleanup(func() { writeGenericCredential = oldWrite })
	writeGenericCredential = func(*wincred.GenericCredential) error {
		return errors.New("access denied")
	}

	if err := SetSecret(id, secret); err == nil {
		t.Fatal("expected SetSecret to fail when wincred write fails")
	}
	if _, err := readDPAPISecret(id); err == nil {
		t.Fatal("DPAPI secret should not exist when wincred write fails")
	}
	if HasSecret(id) {
		t.Fatal("HasSecret should be false when neither store succeeded")
	}
}
