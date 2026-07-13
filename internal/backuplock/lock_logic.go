package backuplock

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"pbs-win-backup/internal/i18nconfig"
)

// LockMaxAge is the maximum time a lock may be held before age-based expiry,
// even when the holder PID appears alive (e.g. PID reuse or ACCESS_DENIED).
const LockMaxAge = 25 * time.Hour

type lockInfo struct {
	pid       int
	timestamp int64
	raw       []byte
}

func parseLockContent(data []byte) (lockInfo, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return lockInfo{raw: data}, i18nconfig.FromConfig().E("lock.empty_file")
	}
	parts := strings.Fields(string(trimmed))
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return lockInfo{raw: data}, i18nconfig.FromConfig().E("lock.invalid_pid")
	}
	var ts int64
	if len(parts) > 1 {
		ts, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return lockInfo{raw: data}, err
		}
	}
	return lockInfo{pid: pid, timestamp: ts, raw: append([]byte(nil), data...)}, nil
}

func lockExpired(info lockInfo, now time.Time, alive func(pid int) bool) bool {
	if info.pid <= 0 {
		return true
	}
	ts := info.timestamp
	if ts <= 0 {
		ts = 0
	}
	if now.Sub(time.Unix(ts, 0)) > LockMaxAge {
		return true
	}
	if alive(info.pid) {
		return false
	}
	return true
}

func lockContentsMatch(a, b []byte) bool {
	return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}

// LockExpiredFromBytes reports whether lock file contents are stale by PID or age.
func LockExpiredFromBytes(data []byte, now time.Time, alive func(pid int) bool) bool {
	info, err := parseLockContent(data)
	if err != nil {
		return true
	}
	return lockExpired(info, now, alive)
}

// IsContention reports whether err indicates another backup holds the lock.
func IsContention(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "held_by_process") || strings.Contains(msg, "lock.held")
}
