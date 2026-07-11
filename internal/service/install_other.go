//go:build !windows

package service

import "pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"

func Install() error   { return i18nconfig.FromConfig().E("platform.windows_only") }
func Uninstall() error { return i18nconfig.FromConfig().E("platform.windows_only") }
func Start() error    { return i18nconfig.FromConfig().E("platform.windows_only") }
func Stop() error     { return i18nconfig.FromConfig().E("platform.windows_only") }
func Restart() error  { return i18nconfig.FromConfig().E("platform.windows_only") }
