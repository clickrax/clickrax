package main

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type restoreController struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	ctx    context.Context
	active int
}

func (c *restoreController) begin(parent context.Context) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == 0 {
		c.ctx, c.cancel = context.WithCancel(parent)
	}
	c.active++
	return c.ctx
}

func (c *restoreController) end() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active > 0 {
		c.active--
	}
	if c.active == 0 && c.cancel != nil {
		c.cancel()
		c.cancel = nil
		c.ctx = nil
	}
}

func (c *restoreController) stop() bool {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (c *restoreController) isRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active > 0
}

func (a *App) beginRestore() context.Context {
	if a.restore == nil {
		a.restore = &restoreController{}
	}
	return a.restore.begin(a.ctx)
}

func (a *App) endRestore() {
	if a.restore != nil {
		a.restore.end()
	}
}

// CancelRestore stops an in-progress file/folder/batch restore.
func (a *App) CancelRestore() {
	if a.restore == nil || !a.restore.stop() {
		return
	}
	runtime.EventsEmit(a.ctx, "restore-progress", map[string]interface{}{
		"files_done":   0,
		"files_total":  0,
		"current_path": a.bundle().T("restore.cancelling"),
		"percent":      0.0,
		"cancelled":    true,
	})
}

// IsRestoreRunning reports whether a restore operation is active.
func (a *App) IsRestoreRunning() bool {
	if a.restore == nil {
		return false
	}
	return a.restore.isRunning()
}

func restoreStatusFromError(err error) (status, msg string) {
	if err == nil {
		return "ok", ""
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", "Восстановление отменено"
	}
	return "error", err.Error()
}

func restoreStatusFromMessage(ok bool, message string) (status, msg string) {
	if ok {
		return "ok", ""
	}
	if message == "Восстановление отменено" {
		return "cancelled", message
	}
	return "error", message
}
