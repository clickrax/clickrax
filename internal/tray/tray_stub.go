//go:build !windows

package tray

import "context"

// NavItem is a tray submenu entry that opens a GUI route.
type NavItem struct {
	Label string
	Path  string
}

// Options configures the system tray (no-op on non-Windows builds).
type Options struct {
	Tooltip    string
	ShowLabel  string
	Nav        []NavItem
	QuitLabel  string
	OnNavigate func(path string)
	OnQuit     func()
}

// Start is a no-op on non-Windows platforms.
func Start(context.Context, Options) {}

// Active reports whether the tray has been started.
func Active() bool { return false }

// ShowWindow is a no-op on non-Windows platforms.
func ShowWindow() {}

// SetTooltip updates the tray hover text (no-op on non-Windows).
func SetTooltip(string) {}

// Shutdown is a no-op on non-Windows platforms.
func Shutdown() {}
