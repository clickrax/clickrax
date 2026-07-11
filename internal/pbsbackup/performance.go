package pbsbackup

import (
	"runtime"
	"sync/atomic"
)

const (
	// Buffered restore writes reduce syscall overhead on Windows.
	restoreWriteBufferSize = 1 << 20 // 1 MiB

	minChunkWorkers = 12
	maxChunkWorkers = 32
)

var chunkWorkersSetting atomic.Int32

// SetChunkWorkersSetting stores user override from settings (0 = auto).
func SetChunkWorkersSetting(n int) {
	if n < 0 {
		n = 0
	}
	chunkWorkersSetting.Store(int32(n))
}

// ChunkWorkers returns parallel HTTP/2 chunk transfers for backup/restore.
// Auto mode targets high-bandwidth links (10 Gbit+): 12–32 workers from CPU count.
func ChunkWorkers() int {
	return effectiveChunkWorkers(int(chunkWorkersSetting.Load()))
}

func effectiveChunkWorkers(configured int) int {
	if configured > 0 {
		if configured > maxChunkWorkers {
			return maxChunkWorkers
		}
		return configured
	}
	n := runtime.NumCPU() * 3
	if n < minChunkWorkers {
		n = minChunkWorkers
	}
	if n > maxChunkWorkers {
		n = maxChunkWorkers
	}
	return n
}
