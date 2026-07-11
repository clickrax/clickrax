//go:build windows

package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"pbs-win-backup/internal/winutil"
)

var sharedAccessOnce sync.Once

// RestrictSensitiveACL limits read/write on config, queue, and history to SYSTEM, Administrators, and the current user.
func RestrictSensitiveACL(path string) error {
	_ = winutil.HiddenCommand("icacls", path, "/inheritance:r").Run()
	_ = winutil.HiddenCommand("icacls", path, "/grant", "*S-1-5-18:F").Run()
	_ = winutil.HiddenCommand("icacls", path, "/grant", "*S-1-5-32-544:F").Run()
	if user := os.Getenv("USERNAME"); user != "" {
		if domain := os.Getenv("USERDOMAIN"); domain != "" {
			grant := fmt.Sprintf("%s\\%s:(F)", domain, user)
			if err := winutil.HiddenCommand("icacls", path, "/grant", grant).Run(); err != nil {
				_ = winutil.HiddenCommand("icacls", path, "/grant", user+":(F)").Run()
			}
		} else {
			_ = winutil.HiddenCommand("icacls", path, "/grant", user+":(F)").Run()
		}
	}
	return nil
}

// AtomicWriteSensitive writes data and applies RestrictSensitiveACL.
func AtomicWriteSensitive(path string, data []byte, perm os.FileMode) error {
	if err := AtomicWrite(path, data, perm); err != nil {
		return err
	}
	return RestrictSensitiveACL(path)
}

// GrantUsersModify allows interactive user backups to update shared operational files.
func GrantUsersModify(path string) error {
	cmd := winutil.HiddenCommand("icacls", path, "/grant", "*S-1-5-32-545:(M)")
	if err := cmd.Run(); err != nil {
		cmd = winutil.HiddenCommand("icacls", path, "/grant", "BUILTIN\\Users:(M)")
		return cmd.Run()
	}
	return nil
}

func grantUsersModifyTree(path string) {
	cmd := winutil.HiddenCommand("icacls", path, "/grant", "*S-1-5-32-545:(OI)(CI)M", "/T")
	if err := cmd.Run(); err != nil {
		_ = winutil.HiddenCommand("icacls", path, "/grant", "BUILTIN\\Users:(OI)(CI)M", "/T").Run()
	}
}

func grantCurrentUserTree(path string) {
	user := os.Getenv("USERNAME")
	if user == "" {
		return
	}
	if domain := os.Getenv("USERDOMAIN"); domain != "" {
		grant := fmt.Sprintf("%s\\%s:(OI)(CI)F", domain, user)
		if err := winutil.HiddenCommand("icacls", path, "/grant", grant).Run(); err != nil {
			_ = winutil.HiddenCommand("icacls", path, "/grant", user+":(OI)(CI)F").Run()
		}
		return
	}
	_ = winutil.HiddenCommand("icacls", path, "/grant", user+":(OI)(CI)F").Run()
}

// ensureSecretsACL restricts secrets/ so Users can create files but not read others' DPAPI blobs.
func ensureSecretsACL(dataDir string) {
	secrets := filepath.Join(dataDir, "secrets")
	if err := os.MkdirAll(secrets, 0o700); err != nil {
		return
	}
	_ = winutil.HiddenCommand("icacls", secrets, "/inheritance:r").Run()
	_ = winutil.HiddenCommand("icacls", secrets, "/grant", "*S-1-5-18:(OI)(CI)F").Run()
	_ = winutil.HiddenCommand("icacls", secrets, "/grant", "*S-1-5-32-544:(OI)(CI)F").Run()
	_ = winutil.HiddenCommand("icacls", secrets, "/grant", "*S-1-5-32-545:(OI)(CI)(GW)").Run()
	if err := winutil.HiddenCommand("icacls", secrets, "/grant", "BUILTIN\\Users:(OI)(CI)(GW)").Run(); err != nil {
		_ = winutil.HiddenCommand("icacls", secrets, "/grant", "BUILTIN\\Users:(OI)(CI)W").Run()
	}
	grantCurrentUserTree(secrets)
}

// ensureServiceSecretsACL restricts service/ secrets to SYSTEM and Administrators only.
func ensureServiceSecretsACL(dataDir string) {
	service := filepath.Join(dataDir, "secrets", "service")
	if err := os.MkdirAll(service, 0o700); err != nil {
		return
	}
	_ = winutil.HiddenCommand("icacls", service, "/inheritance:r").Run()
	_ = winutil.HiddenCommand("icacls", service, "/grant", "*S-1-5-18:(OI)(CI)F").Run()
	_ = winutil.HiddenCommand("icacls", service, "/grant", "*S-1-5-32-544:(OI)(CI)F").Run()
	grantCurrentUserTree(service)
}

// EnsureSharedDataAccess grants Users modify on shared operational subdirs (not secrets/ or sensitive root files).
func EnsureSharedDataAccess() {
	sharedAccessOnce.Do(func() {
		dir, err := dataDirNoACL()
		if err != nil {
			return
		}
		for _, sub := range []string{"logs", "index", "checkpoints"} {
			subPath := filepath.Join(dir, sub)
			if err := os.MkdirAll(subPath, 0o755); err != nil {
				continue
			}
			grantUsersModifyTree(subPath)
		}
		ensureSecretsACL(dir)
		ensureServiceSecretsACL(dir)
		ensureCancelDirACL(dir)
	})
}

// ensureCancelDirACL lets SYSTEM/service and the installing user manage cancel IPC files.
func ensureCancelDirACL(dataDir string) {
	cancel := filepath.Join(dataDir, "cancel")
	if err := os.MkdirAll(cancel, 0o755); err != nil {
		return
	}
	_ = winutil.HiddenCommand("icacls", cancel, "/inheritance:r").Run()
	_ = winutil.HiddenCommand("icacls", cancel, "/grant", "*S-1-5-18:(OI)(CI)F").Run()
	_ = winutil.HiddenCommand("icacls", cancel, "/grant", "*S-1-5-32-544:(OI)(CI)F").Run()
	grantCurrentUserTree(cancel)
}
