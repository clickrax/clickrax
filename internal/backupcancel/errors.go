package backupcancel

import "errors"

var (
	errEmptyJobID   = errors.New("backup cancel: empty job id")
	errInvalidJobID = errors.New("backup cancel: invalid job id")
)
