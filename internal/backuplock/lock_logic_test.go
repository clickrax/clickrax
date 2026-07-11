package backuplock

import (
	"testing"
	"time"
)

func TestLockExpired_AgeFallbackEvenWhenAlive(t *testing.T) {
	old := time.Now().Unix() - int64((LockMaxAge + time.Hour).Seconds())
	info := lockInfo{pid: 4242, timestamp: old}
	if !lockExpired(info, time.Now(), func(int) bool { return true }) {
		t.Fatal("expected age-based expiry even when PID appears alive")
	}
}

func TestLockExpired_RecentAliveNotExpired(t *testing.T) {
	info := lockInfo{pid: 4242, timestamp: time.Now().Unix()}
	if lockExpired(info, time.Now(), func(int) bool { return true }) {
		t.Fatal("recent alive lock should not expire")
	}
}
