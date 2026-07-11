package backupqueue

import "errors"

// Drain pops and starts the next queued backup when the runner is idle and the lock is free.
func Drain(isRunning func() bool, canStart func() bool, start func(Item) error) {
	if isRunning == nil || canStart == nil || start == nil {
		return
	}
	if isRunning() || !canStart() {
		return
	}
	item, ok, err := ClaimNext()
	if err != nil || !ok {
		return
	}
	item.FromQueue = true
	if err := start(item); err != nil {
		if IsPermanentStartError(err) {
			_ = clearInflight()
			RecordDeadLetter(item, err)
			return
		}
		if requeueErr := enqueueFront(item); requeueErr != nil {
			_ = clearInflight()
			RecordDeadLetter(item, requeueErr)
			return
		}
		_ = clearInflight()
		return
	}
}

// ClearInflight removes the durable in-flight marker after terminal queue handling.
func ClearInflight() error {
	return clearInflight()
}

// ErrQueuePanic is returned when a queued backup panics.
var ErrQueuePanic = errors.New("queued backup panicked")

var enqueueFrontHook func(Item) error

func enqueueFront(item Item) error {
	if enqueueFrontHook != nil {
		return enqueueFrontHook(item)
	}
	return EnqueueFront(item)
}
