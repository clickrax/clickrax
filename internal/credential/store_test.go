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

func TestGetPassphrase_DPAPIWriteFails_KeepsWincred(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", old) }()

	jobID := uuid.NewString()
	pass := "encryption-passphrase"
	cred := wincred.NewGenericCredential(passphraseTarget(jobID))
	cred.CredentialBlob = []byte(pass)
	cred.Persist = wincred.PersistSession
	if err := writeGenericCredential(cred); err != nil {
		t.Fatal(err)
	}

	oldWrite := writeDPAPIPassphrase
	t.Cleanup(func() { writeDPAPIPassphrase = oldWrite })
	writeDPAPIPassphrase = func(string, string) error {
		return errors.New("dpapi write failed")
	}

	got, err := GetPassphrase(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if got != pass {
		t.Fatalf("got %q want %q", got, pass)
	}
	if _, err := wincred.GetGenericCredential(passphraseTarget(jobID)); err != nil {
		t.Fatal("wincred passphrase should remain when DPAPI migration fails")
	}
}

func TestMigratePassphrases_DPAPIWriteFails_KeepsWincred(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", old) }()

	jobID := uuid.NewString()
	pass := "migrate-passphrase"
	cred := wincred.NewGenericCredential(passphraseTarget(jobID))
	cred.CredentialBlob = []byte(pass)
	cred.Persist = wincred.PersistSession
	if err := writeGenericCredential(cred); err != nil {
		t.Fatal(err)
	}

	oldWrite := writeDPAPIPassphrase
	t.Cleanup(func() { writeDPAPIPassphrase = oldWrite })
	writeDPAPIPassphrase = func(string, string) error {
		return errors.New("dpapi write failed")
	}

	MigratePassphrases([]string{jobID})

	if _, err := readDPAPIPassphrase(jobID); err == nil {
		t.Fatal("DPAPI file should not exist when write failed")
	}
	if got, err := wincred.GetGenericCredential(passphraseTarget(jobID)); err != nil || string(got.CredentialBlob) != pass {
		t.Fatal("wincred passphrase should remain when migration write fails")
	}
}
