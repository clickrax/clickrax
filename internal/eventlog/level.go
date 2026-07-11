package eventlog

import "strings"

type Level int

const (
	LevelError Level = iota
	LevelWarning
	LevelInfo
	LevelDebug
)

var currentLevel = LevelInfo

// SetLevel configures minimum severity from config ("error", "warn", "info", "debug").
func SetLevel(s string) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		currentLevel = LevelDebug
	case "warn", "warning":
		currentLevel = LevelWarning
	case "error":
		currentLevel = LevelError
	default:
		currentLevel = LevelInfo
	}
}

func Debug(msg string) {
	if currentLevel < LevelDebug {
		return
	}
	Info(msg)
}
