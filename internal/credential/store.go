package credential

import (
	"errors"
	"log"
	"runtime"
	"strings"

	"pbs-win-backup/internal/i18nconfig"

	"github.com/danieljoos/wincred"
	"github.com/google/uuid"
)

const targetPrefix = "PbsWinBackup:"

func secretTarget(serverID string) string {
	return targetPrefix + "secret:" + serverID
}

func passphraseTarget(jobID string) string {
	return targetPrefix + "passphrase:" + jobID
}

func smtpPasswordTarget() string {
	return targetPrefix + "smtp:password"
}

func validateCredentialID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return i18nconfig.FromConfig().E("cred.invalid_id")
	}
	if _, err := uuid.Parse(id); err != nil {
		return i18nconfig.FromConfig().E("cred.invalid_id")
	}
	return nil
}

func deleteWincredSecret(serverID string) {
	cred, err := wincred.GetGenericCredential(secretTarget(serverID))
	if err != nil {
		return
	}
	_ = cred.Delete()
}

func deleteWincredSMTP() {
	cred, err := wincred.GetGenericCredential(smtpPasswordTarget())
	if err != nil {
		return
	}
	_ = cred.Delete()
}

func SetSecret(serverID, secret string) error {
	if runtime.GOOS != "windows" {
		return i18nconfig.FromConfig().E("cred.windows_only")
	}
	if err := validateCredentialID(serverID); err != nil {
		return err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return i18nconfig.FromConfig().E("cred.empty_secret")
	}
	cred := wincred.NewGenericCredential(secretTarget(serverID))
	cred.CredentialBlob = []byte(secret)
	cred.Persist = wincred.PersistSession
	if err := writeGenericCredential(cred); err != nil {
		return errors.Join(
			i18nconfig.FromConfig().Ef("cred.save_failed", map[string]string{"err": err.Error()}),
			err,
		)
	}
	if err := writeDPAPISecret(serverID, secret); err != nil {
		deleteWincredSecret(serverID)
		return i18nconfig.FromConfig().Ewrap("cred.save_secret_service", nil, err)
	}
	deleteWincredSecret(serverID)
	return nil
}

func GetSecret(serverID string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", i18nconfig.FromConfig().E("cred.windows_only")
	}
	if err := validateCredentialID(serverID); err != nil {
		return "", err
	}
	if s, err := readDPAPISecret(serverID); err == nil {
		return strings.TrimSpace(s), nil
	}
	cred, err := wincred.GetGenericCredential(secretTarget(serverID))
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(cred.CredentialBlob))
	if secret == "" {
		return "", i18nconfig.FromConfig().E("cred.empty_secret_stored")
	}
	if err := writeDPAPISecret(serverID, secret); err != nil {
		log.Printf("credential: lazy DPAPI migration failed for server %s: %v", serverID, err)
	} else {
		deleteWincredSecret(serverID)
	}
	return secret, nil
}

func DeleteSecret(serverID string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if err := validateCredentialID(serverID); err != nil {
		return err
	}
	deleteDPAPISecret(serverID)
	cred, err := wincred.GetGenericCredential(secretTarget(serverID))
	if err != nil {
		return nil
	}
	return cred.Delete()
}

func DeletePassphrase(jobID string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if err := validateCredentialID(jobID); err != nil {
		return err
	}
	deletePassphraseStores(jobID)
	return nil
}

func SetPassphrase(jobID, passphrase string) error {
	if runtime.GOOS != "windows" {
		return i18nconfig.FromConfig().E("cred.windows_only")
	}
	if err := validateCredentialID(jobID); err != nil {
		return err
	}
	if err := writeDPAPIPassphrase(jobID, passphrase); err != nil {
		return err
	}
	deleteWincredPassphrase(jobID)
	return nil
}

func deleteWincredPassphrase(jobID string) {
	cred, err := wincred.GetGenericCredential(passphraseTarget(jobID))
	if err != nil {
		return
	}
	_ = cred.Delete()
}

func deletePassphraseStores(jobID string) {
	deleteDPAPIPassphrase(jobID)
	deleteWincredPassphrase(jobID)
}

func GetPassphrase(jobID string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", i18nconfig.FromConfig().E("cred.windows_only")
	}
	if err := validateCredentialID(jobID); err != nil {
		return "", err
	}
	if s, err := readDPAPIPassphrase(jobID); err == nil {
		return s, nil
	}
	cred, err := wincred.GetGenericCredential(passphraseTarget(jobID))
	if err != nil {
		return "", err
	}
	pass := string(cred.CredentialBlob)
	if err := writeDPAPIPassphrase(jobID, pass); err != nil {
		log.Printf("credential: lazy DPAPI passphrase migration failed for job %s: %v", jobID, err)
	} else {
		_ = cred.Delete()
	}
	return pass, nil
}

func SetSMTPPassword(password string) error {
	if runtime.GOOS != "windows" {
		return i18nconfig.FromConfig().E("cred.windows_only")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return i18nconfig.FromConfig().E("cred.empty_smtp")
	}
	cred := wincred.NewGenericCredential(smtpPasswordTarget())
	cred.CredentialBlob = []byte(password)
	cred.Persist = wincred.PersistSession
	if err := writeGenericCredential(cred); err != nil {
		return errors.Join(
			i18nconfig.FromConfig().Ef("cred.save_failed", map[string]string{"err": err.Error()}),
			err,
		)
	}
	if err := writeDPAPISMTPPassword(password); err != nil {
		deleteWincredSMTP()
		return i18nconfig.FromConfig().Ewrap("cred.save_smtp_service", nil, err)
	}
	deleteWincredSMTP()
	return nil
}

func GetSMTPPassword() (string, error) {
	if runtime.GOOS != "windows" {
		return "", i18nconfig.FromConfig().E("cred.windows_only")
	}
	if s, err := readDPAPISMTPPassword(); err == nil {
		return strings.TrimSpace(s), nil
	}
	cred, err := wincred.GetGenericCredential(smtpPasswordTarget())
	if err != nil {
		return "", err
	}
	password := strings.TrimSpace(string(cred.CredentialBlob))
	if password == "" {
		return "", i18nconfig.FromConfig().E("cred.empty_smtp_stored")
	}
	if err := writeDPAPISMTPPassword(password); err != nil {
		log.Printf("credential: lazy DPAPI SMTP migration failed: %v", err)
	} else {
		deleteWincredSMTP()
	}
	return password, nil
}

func HasSMTPPassword() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if _, err := readDPAPISMTPPassword(); err == nil {
		return true
	}
	_, err := GetSMTPPassword()
	return err == nil
}
