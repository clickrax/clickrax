//go:build !windows

package service

import "pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"

func RunService(isDebug bool) error {
	return i18nconfig.FromConfig().E("platform.service_windows_only")
}

func Install() error   { return i18nconfig.FromConfig().E("platform.windows_only") }
func Uninstall() error { return i18nconfig.FromConfig().E("platform.windows_only") }

func IsWindowsServiceProcess() bool { return false }
