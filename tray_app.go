package main

import (
	"context"
	"fmt"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const trayTooltipMaxRunes = 120

func (a *App) trayNavItems() []tray.NavItem {
	b := a.bundle()
	return []tray.NavItem{
		{Label: b.T("tray.nav_dashboard"), Path: "/"},
		{Label: b.T("tray.nav_servers"), Path: "/servers"},
		{Label: b.T("tray.nav_jobs"), Path: "/jobs"},
		{Label: b.T("tray.nav_progress"), Path: "/progress"},
		{Label: b.T("tray.nav_restore"), Path: "/restore"},
		{Label: b.T("tray.nav_logs"), Path: "/logs"},
		{Label: b.T("tray.nav_settings"), Path: "/settings"},
		{Label: b.T("tray.nav_diagnostics"), Path: "/diagnostics"},
	}
}

func (a *App) ensureTray() {
	if a.ctx == nil || !a.store.Settings().MinimizeToTray {
		return
	}
	b := a.bundle()
	a.mu.RLock()
	lp := a.lastProgress
	a.mu.RUnlock()
	tray.Start(a.ctx, tray.Options{
		Tooltip:    a.trayTooltipText(lp),
		ShowLabel:  b.T("tray.show"),
		Nav:        a.trayNavItems(),
		QuitLabel:  b.T("tray.quit"),
		OnNavigate: a.trayNavigate,
		OnQuit:     a.quitFromTray,
	})
}

func (a *App) updateTrayTooltip() {
	if a.ctx == nil || !a.store.Settings().MinimizeToTray || !tray.Active() {
		return
	}
	a.mu.RLock()
	lp := a.lastProgress
	a.mu.RUnlock()
	tray.SetTooltip(a.trayTooltipText(lp))
}

func (a *App) trayTooltipText(lp models.ProgressEvent) string {
	b := a.bundle()

	if a.IsStopping() {
		job := lp.JobName
		if job == "" {
			job = branding.Name
		}
		return truncTrayTooltip(b.Tf("tray.tooltip_stopping", map[string]string{"job": job}))
	}

	if isInProgressPhase(lp.Phase) || (a.engine != nil && a.engine.IsRunning() && !isTerminalPhase(lp.Phase)) {
		job := lp.JobName
		if job == "" {
			job = branding.Name
		}
		pct := lp.Percent
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		return truncTrayTooltip(b.Tf("tray.tooltip_running", map[string]string{
			"job":    job,
			"detail": trayPhaseDetail(b, lp),
			"pct":    fmt.Sprintf("%.0f", pct),
		}))
	}

	if lp.JobID != "" && a.isJobActivelyRunning(lp.JobID) {
		job := lp.JobName
		if job == "" {
			job = branding.Name
		}
		return truncTrayTooltip(b.Tf("tray.tooltip_running", map[string]string{
			"job":    job,
			"detail": b.T("tray.phase_active"),
			"pct":    fmt.Sprintf("%.0f", lp.Percent),
		}))
	}

	return truncTrayTooltip(b.T("tray.tooltip_idle"))
}

func trayPhaseDetail(b *i18n.Bundle, ev models.ProgressEvent) string {
	if ev.Message != "" {
		switch ev.Phase {
		case models.PhaseError, models.PhaseCancelled, models.PhaseDone:
			return ev.Message
		}
	}
	key := ""
	switch ev.Phase {
	case models.PhasePreparing:
		key = "tray.phase_preparing"
	case models.PhaseAnalyzing:
		key = "tray.phase_analyzing"
	case models.PhaseVSS:
		key = "tray.phase_vss"
	case models.PhaseTransfer:
		key = "tray.phase_transfer"
	case models.PhaseFinalizing:
		key = "tray.phase_finalizing"
	case models.PhaseVerify:
		key = "tray.phase_verify"
	default:
		if ev.Message != "" {
			return ev.Message
		}
		key = "tray.phase_active"
	}
	return b.T(key)
}

func truncTrayTooltip(s string) string {
	r := []rune(s)
	if len(r) <= trayTooltipMaxRunes {
		return s
	}
	return string(r[:trayTooltipMaxRunes-1]) + "…"
}

func (a *App) trayNavigate(path string) {
	if a.ctx == nil || path == "" {
		return
	}
	runtime.EventsEmit(a.ctx, "navigate", path)
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.forceQuit || !a.store.Settings().MinimizeToTray {
		return false
	}
	a.ensureTray()
	runtime.WindowHide(ctx)
	a.updateTrayTooltip()
	return true
}

func (a *App) quitFromTray() {
	a.forceQuit = true
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}
