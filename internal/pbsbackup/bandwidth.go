package pbsbackup

import (
	"sync"
	"time"
)

type bandwidthLimiter struct {
	bytesPerSec int64
	mu          sync.Mutex
}

func newBandwidthLimiter(bytesPerSec int64) *bandwidthLimiter {
	if bytesPerSec <= 0 {
		return nil
	}
	return &bandwidthLimiter{bytesPerSec: bytesPerSec}
}

func (b *bandwidthLimiter) wait(n int) {
	if b == nil || b.bytesPerSec <= 0 || n <= 0 {
		return
	}
	delay := time.Duration(float64(n) / float64(b.bytesPerSec) * float64(time.Second))
	if delay > 0 {
		time.Sleep(delay)
	}
}
