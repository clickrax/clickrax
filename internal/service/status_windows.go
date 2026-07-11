//go:build windows

package service

import (
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"
	"golang.org/x/sys/windows/svc"
)

func QueryStatus() StatusInfo {
	return QueryStatusLang(i18nconfig.FromConfig().Lang())
}

func QueryStatusLang(lang string) StatusInfo {
	b := i18n.New(lang)
	m, err := openManager()
	if err != nil {
		return StatusInfo{Message: err.Error()}
	}
	defer m.Disconnect()

	s, err := openService(m)
	if err != nil {
		if isMarkedForDeletion(err) {
			return StatusInfo{
				Installed:     true,
				PendingDelete: true,
				State:         "deleting",
				Message:       b.T("service.state.deleting"),
			}
		}
		return StatusInfo{
			Installed: false,
			State:     "not_installed",
			Message:   b.T("service.state.not_installed"),
		}
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return StatusInfo{Installed: true, Message: err.Error()}
	}

	stateName := stateString(status.State)
	return StatusInfo{
		Installed: true,
		Running:   status.State == svc.Running,
		State:     stateName,
		Message:   stateLabelLang(status.State, b),
	}
}

func stateString(st svc.State) string {
	switch st {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

func stateLabelLang(st svc.State, b *i18n.Bundle) string {
	switch st {
	case svc.Running:
		return b.T("service.state.running")
	case svc.Stopped:
		return b.T("service.state.stopped")
	case svc.StartPending:
		return b.T("service.state.start_pending")
	case svc.StopPending:
		return b.T("service.state.stop_pending")
	default:
		return stateString(st)
	}
}
