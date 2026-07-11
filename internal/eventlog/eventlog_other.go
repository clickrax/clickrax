//go:build !windows

package eventlog

func Info(msg string) {
	if currentLevel < LevelInfo {
		return
	}
}

func Warning(msg string) {
	if currentLevel < LevelWarning {
		return
	}
}

func Error(string) {}

func Debug(msg string) {
	if currentLevel < LevelDebug {
		return
	}
}
