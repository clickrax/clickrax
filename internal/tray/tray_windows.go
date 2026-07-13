//go:build windows

package tray

import (
	"context"
	_ "embed"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"

	"pbs-win-backup/internal/branding"
)

//go:embed icon.ico
var iconData []byte

// NavItem is a tray submenu entry that opens a GUI route.
type NavItem struct {
	Label string
	Path  string
}

// Options configures the system tray.
type Options struct {
	Tooltip    string
	ShowLabel  string
	Nav        []NavItem
	QuitLabel  string
	OnNavigate func(path string)
	OnQuit     func()
}

var (
	startMu      sync.Mutex
	started      bool
	ready        atomic.Bool
	appCtx       context.Context
	lastTooltip  string
	tooltipMu    sync.Mutex
	pendingTip   string
	shutdownOnce sync.Once

	tooltipScheduleMu sync.Mutex
	tooltipScheduled  bool
	lastTooltipAt     time.Time
)

const tooltipMinInterval = 400 * time.Millisecond

// Start launches the Windows notification-area icon (idempotent).
// Call only after the user hides the window to tray — not at app startup.
func Start(ctx context.Context, opt Options) {
	if ctx == nil {
		return
	}
	startMu.Lock()
	if started {
		startMu.Unlock()
		if opt.Tooltip != "" {
			SetTooltip(opt.Tooltip)
		}
		return
	}
	started = true
	appCtx = ctx
	if opt.Tooltip != "" {
		pendingTip = opt.Tooltip
	}
	startMu.Unlock()

	go systray.Run(func() { onReady(opt) }, func() {})
}

// Active reports whether the tray icon has been created.
func Active() bool {
	startMu.Lock()
	defer startMu.Unlock()
	return started
}

func onReady(opt Options) {
	if len(iconData) > 0 {
		systray.SetIcon(iconData)
	}
	ready.Store(true)
	applyTooltip(pendingTip)
	systray.OnLeftClick(ShowWindow)

	show := systray.AddMenuItem(opt.ShowLabel, opt.ShowLabel)
	go func() {
		for range show.ClickedCh {
			ShowWindow()
		}
	}()

	if len(opt.Nav) > 0 {
		systray.AddSeparator()
		for _, item := range opt.Nav {
			item := item
			mi := systray.AddMenuItem(item.Label, item.Label)
			go func() {
				for range mi.ClickedCh {
					ShowWindow()
					if item.Path != "" && opt.OnNavigate != nil {
						opt.OnNavigate(item.Path)
					}
				}
			}()
		}
	}

	systray.AddSeparator()
	quit := systray.AddMenuItem(opt.QuitLabel, opt.QuitLabel)
	go func() {
		for range quit.ClickedCh {
			if opt.OnQuit != nil {
				opt.OnQuit()
			}
		}
	}()
}

func applyTooltip(text string) {
	if text == "" || !ready.Load() {
		return
	}
	tooltipMu.Lock()
	if text == lastTooltip {
		tooltipMu.Unlock()
		return
	}
	lastTooltip = text
	tooltipMu.Unlock()
	systray.SetTooltip(text)
}

// ShowWindow restores the main Wails window.
// systray already invokes left-click handlers in a goroutine (see vendor systrayIconLeftClicked).
func ShowWindow() {
	if appCtx == nil {
		showWindowNative()
		return
	}
	runtime.WindowShow(appCtx)
	showWindowNative()
}

var (
	user32DLL          = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows    = user32DLL.NewProc("EnumWindows")
	procShowWindowWin  = user32DLL.NewProc("ShowWindow")
	procSetForeground  = user32DLL.NewProc("SetForegroundWindow")
	procGetWindowTextW = user32DLL.NewProc("GetWindowTextW")
)

const swShow = 5

type enumWindowData struct {
	prefix string
	found  windows.Handle
}

func showWindowNative() {
	data := enumWindowData{prefix: branding.Title}
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		d := (*enumWindowData)(unsafe.Pointer(lParam))
		var buf [256]uint16
		n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return 1
		}
		title := windows.UTF16ToString(buf[:])
		if len(title) >= len(d.prefix) && title[:len(d.prefix)] == d.prefix {
			d.found = windows.Handle(hwnd)
			return 0
		}
		return 1
	})
	procEnumWindows.Call(cb, uintptr(unsafe.Pointer(&data)))
	if data.found == 0 {
		return
	}
	_, _, _ = procShowWindowWin.Call(uintptr(data.found), swShow)
	_, _, _ = procSetForeground.Call(uintptr(data.found))
}

func scheduleTooltipUpdate() {
	tooltipScheduleMu.Lock()
	if tooltipScheduled {
		tooltipScheduleMu.Unlock()
		return
	}
	tooltipScheduled = true
	tooltipScheduleMu.Unlock()

	go func() {
		defer func() {
			tooltipScheduleMu.Lock()
			tooltipScheduled = false
			tooltipScheduleMu.Unlock()
		}()
		if since := time.Since(lastTooltipAt); since < tooltipMinInterval {
			time.Sleep(tooltipMinInterval - since)
		}
		text := pendingTip
		if text == "" || !ready.Load() {
			return
		}
		applyTooltip(text)
		lastTooltipAt = time.Now()
	}()
}

// SetTooltip updates the notification-area hover text (safe before/after ready).
func SetTooltip(text string) {
	if text == "" {
		return
	}
	pendingTip = text
	if ready.Load() {
		scheduleTooltipUpdate()
	}
}

// Shutdown removes the tray icon during application exit.
func Shutdown() {
	shutdownOnce.Do(func() {
		if !started {
			return
		}
		ready.Store(false)
		systray.Quit()
	})
}
