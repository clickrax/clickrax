//go:build windows

package credential

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"

	"github.com/danieljoos/wincred"
)

var (
	modCrypt32             = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
	cryptLocalMachine      = uintptr(0x4)
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func secretsDir() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return p, nil
}

func userSecretsDir() (string, error) {
	base, err := secretsDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, "user")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return p, nil
}

func serviceSecretsDir() (string, error) {
	base, err := secretsDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, "service")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return p, nil
}

// secretFilePath returns the user-scoped DPAPI path (used by tests and diagnostics).
func secretFilePath(serverID string) (string, error) {
	return userSecretFilePath(serverID)
}

func legacySecretFilePath(serverID string) (string, error) {
	if err := validateCredentialID(serverID); err != nil {
		return "", err
	}
	dir, err := secretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serverID+".dpapi"), nil
}

func userSecretFilePath(serverID string) (string, error) {
	if err := validateCredentialID(serverID); err != nil {
		return "", err
	}
	dir, err := userSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serverID+".dpapi"), nil
}

func serviceSecretFilePath(serverID string) (string, error) {
	if err := validateCredentialID(serverID); err != nil {
		return "", err
	}
	dir, err := serviceSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serverID+".dpapi"), nil
}

func protectData(plain []byte, flags uintptr) ([]byte, error) {
	if len(plain) == 0 {
		return nil, i18nconfig.FromConfig().E("cred.empty_secret")
	}
	in := dataBlob{cbData: uint32(len(plain)), pbData: &plain[0]}
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, flags,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		if err != syscall.Errno(0) {
			return nil, fmt.Errorf("CryptProtectData: %w", err)
		}
		return nil, fmt.Errorf("CryptProtectData failed")
	}
	protected := make([]byte, out.cbData)
	copy(protected, unsafe.Slice(out.pbData, out.cbData))
	localFree := syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")
	_, _, _ = localFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return protected, nil
}

func unprotectData(protected []byte) ([]byte, error) {
	if len(protected) == 0 {
		return nil, i18nconfig.FromConfig().E("cred.empty_data")
	}
	in := dataBlob{cbData: uint32(len(protected)), pbData: &protected[0]}
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		if err != syscall.Errno(0) {
			return nil, fmt.Errorf("CryptUnprotectData: %w", err)
		}
		return nil, fmt.Errorf("CryptUnprotectData failed")
	}
	plain := make([]byte, out.cbData)
	copy(plain, unsafe.Slice(out.pbData, out.cbData))
	_, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.pbData)))
	return plain, nil
}

func writeEncryptedFile(path string, enc []byte, restrict bool) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(enc); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if restrict {
		_ = paths.RestrictSensitiveACL(path)
	}
	return paths.SyncDir(filepath.Dir(path))
}

func readEncryptedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	plain, err := unprotectData(data)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func writeDPAPISecret(serverID, secret string) error {
	userPath, err := userSecretFilePath(serverID)
	if err != nil {
		return err
	}
	userEnc, err := protectData([]byte(secret), 0)
	if err != nil {
		return err
	}
	if err := writeEncryptedFile(userPath, userEnc, false); err != nil {
		return err
	}

	servicePath, err := serviceSecretFilePath(serverID)
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	serviceEnc, err := protectData([]byte(secret), cryptLocalMachine)
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	if err := writeEncryptedFile(servicePath, serviceEnc, true); err != nil {
		_ = os.Remove(userPath)
		return err
	}

	if legacy, err := legacySecretFilePath(serverID); err == nil {
		_ = os.Remove(legacy)
	}
	return nil
}

func readDPAPISecret(serverID string) (string, error) {
	tryPaths := func(paths ...string) (string, error) {
		var lastErr error
		for _, p := range paths {
			s, err := readEncryptedFile(p)
			if err == nil {
				return s, nil
			}
			lastErr = err
		}
		return "", lastErr
	}

	userPath, _ := userSecretFilePath(serverID)
	servicePath, _ := serviceSecretFilePath(serverID)
	legacyPath, _ := legacySecretFilePath(serverID)

	if preferServiceSecrets() {
		return tryPaths(servicePath, userPath, legacyPath)
	}
	return tryPaths(userPath, servicePath, legacyPath)
}

func deleteDPAPISecret(serverID string) {
	for _, fn := range []func(string) (string, error){
		userSecretFilePath, serviceSecretFilePath, legacySecretFilePath,
	} {
		if p, err := fn(serverID); err == nil {
			_ = os.Remove(p)
		}
	}
}

func smtpUserFilePath() (string, error) {
	dir, err := userSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "smtp.dpapi"), nil
}

func smtpServiceFilePath() (string, error) {
	dir, err := serviceSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "smtp.dpapi"), nil
}

func smtpLegacyFilePath() (string, error) {
	dir, err := secretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "smtp.dpapi"), nil
}

func writeDPAPISMTPPassword(password string) error {
	userPath, err := smtpUserFilePath()
	if err != nil {
		return err
	}
	userEnc, err := protectData([]byte(password), 0)
	if err != nil {
		return err
	}
	if err := writeEncryptedFile(userPath, userEnc, false); err != nil {
		return err
	}
	servicePath, err := smtpServiceFilePath()
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	serviceEnc, err := protectData([]byte(password), cryptLocalMachine)
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	if err := writeEncryptedFile(servicePath, serviceEnc, true); err != nil {
		_ = os.Remove(userPath)
		return err
	}
	if legacy, err := smtpLegacyFilePath(); err == nil {
		_ = os.Remove(legacy)
	}
	return nil
}

func readDPAPISMTPPassword() (string, error) {
	userPath, _ := smtpUserFilePath()
	servicePath, _ := smtpServiceFilePath()
	legacyPath, _ := smtpLegacyFilePath()
	if preferServiceSecrets() {
		if s, err := readEncryptedFile(servicePath); err == nil {
			return s, nil
		}
		if s, err := readEncryptedFile(userPath); err == nil {
			return s, nil
		}
		return readEncryptedFile(legacyPath)
	}
	if s, err := readEncryptedFile(userPath); err == nil {
		return s, nil
	}
	if s, err := readEncryptedFile(servicePath); err == nil {
		return s, nil
	}
	return readEncryptedFile(legacyPath)
}

func deleteDPAPISMTPPassword() {
	for _, pathFn := range []func() (string, error){smtpUserFilePath, smtpServiceFilePath, smtpLegacyFilePath} {
		if p, err := pathFn(); err == nil {
			_ = os.Remove(p)
		}
	}
}

func passphraseUserFilePath(jobID string) (string, error) {
	if err := validateCredentialID(jobID); err != nil {
		return "", err
	}
	dir, err := userSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pass_"+jobID+".dpapi"), nil
}

func passphraseServiceFilePath(jobID string) (string, error) {
	if err := validateCredentialID(jobID); err != nil {
		return "", err
	}
	dir, err := serviceSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pass_"+jobID+".dpapi"), nil
}

var writeDPAPIPassphrase = func(jobID, passphrase string) error {
	userPath, err := passphraseUserFilePath(jobID)
	if err != nil {
		return err
	}
	userEnc, err := protectData([]byte(passphrase), 0)
	if err != nil {
		return err
	}
	if err := writeEncryptedFile(userPath, userEnc, false); err != nil {
		return err
	}
	servicePath, err := passphraseServiceFilePath(jobID)
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	serviceEnc, err := protectData([]byte(passphrase), cryptLocalMachine)
	if err != nil {
		_ = os.Remove(userPath)
		return err
	}
	if err := writeEncryptedFile(servicePath, serviceEnc, true); err != nil {
		_ = os.Remove(userPath)
		return err
	}
	return nil
}

func readDPAPIPassphrase(jobID string) (string, error) {
	userPath, _ := passphraseUserFilePath(jobID)
	servicePath, _ := passphraseServiceFilePath(jobID)
	if preferServiceSecrets() {
		if s, err := readEncryptedFile(servicePath); err == nil {
			return s, nil
		}
		return readEncryptedFile(userPath)
	}
	if s, err := readEncryptedFile(userPath); err == nil {
		return s, nil
	}
	return readEncryptedFile(servicePath)
}

func deleteDPAPIPassphrase(jobID string) {
	if p, err := passphraseUserFilePath(jobID); err == nil {
		_ = os.Remove(p)
	}
	if p, err := passphraseServiceFilePath(jobID); err == nil {
		_ = os.Remove(p)
	}
}

// HasSecret reports whether a readable secret exists for the server.
func HasSecret(serverID string) bool {
	_, err := readDPAPISecret(serverID)
	return err == nil
}

// MigrateSecrets copies legacy secrets into user/service DPAPI files.
func MigrateSecrets(serverIDs []string) {
	for _, id := range serverIDs {
		if HasSecret(id) {
			continue
		}
		cred, err := wincred.GetGenericCredential(secretTarget(id))
		if err != nil {
			if legacy, lerr := legacySecretFilePath(id); lerr == nil {
				if s, rerr := readEncryptedFile(legacy); rerr == nil {
					_ = writeDPAPISecret(id, s)
					continue
				}
			}
			continue
		}
		if err := writeDPAPISecret(id, string(cred.CredentialBlob)); err == nil {
			_ = cred.Delete()
		}
	}
}

// MigrateSMTPPassword copies SMTP password into dual-scope DPAPI files.
func MigrateSMTPPassword() {
	if _, err := readDPAPISMTPPassword(); err == nil {
		return
	}
	cred, err := wincred.GetGenericCredential(smtpPasswordTarget())
	if err != nil {
		return
	}
	password := strings.TrimSpace(string(cred.CredentialBlob))
	if password == "" {
		return
	}
	if err := writeDPAPISMTPPassword(password); err == nil {
		deleteWincredSMTP()
	}
}

// MigratePassphrases copies wincred passphrases into DPAPI files.
func MigratePassphrases(jobIDs []string) {
	for _, id := range jobIDs {
		if _, err := readDPAPIPassphrase(id); err == nil {
			continue
		}
		cred, err := wincred.GetGenericCredential(passphraseTarget(id))
		if err != nil {
			continue
		}
		if err := writeDPAPIPassphrase(id, string(cred.CredentialBlob)); err == nil {
			_ = cred.Delete()
		}
	}
}
