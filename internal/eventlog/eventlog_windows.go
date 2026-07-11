//go:build windows

package eventlog

import (
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

const source = "PbsWinBackup"

func ensureSource() {
	err := eventlog.InstallAsEventCreate(source, eventlog.Info|eventlog.Warning|eventlog.Error)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "registry key already exists") {
			return
		}
	}
}

func Info(msg string) {
	if currentLevel < LevelInfo {
		return
	}
	ensureSource()
	elog, err := eventlog.Open(source)
	if err != nil {
		return
	}
	defer elog.Close()
	_ = elog.Info(1, msg)
}

func Warning(msg string) {
	if currentLevel < LevelWarning {
		return
	}
	ensureSource()
	elog, err := eventlog.Open(source)
	if err != nil {
		return
	}
	defer elog.Close()
	_ = elog.Warning(2, msg)
}

func Error(msg string) {
	ensureSource()
	elog, err := eventlog.Open(source)
	if err != nil {
		return
	}
	defer elog.Close()
	_ = elog.Error(3, msg)
}
