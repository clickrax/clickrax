package backupqueue

import (
	"fmt"

	"pbs-win-backup/internal/i18nconfig"
)

const (
	MaxQueueItems        = 1000
	MaxDeadLetterRecords = 500
)

type queueFullError struct{}

func (queueFullError) Error() string {
	return i18nconfig.FromConfig().Tf("queue.full", map[string]string{
		"max": fmt.Sprintf("%d", MaxQueueItems),
	})
}

// ErrQueueFull is returned when the backup queue exceeds MaxQueueItems.
var ErrQueueFull = queueFullError{}
