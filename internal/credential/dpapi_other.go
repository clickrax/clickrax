//go:build !windows

package credential

import "errors"

var errUnavailable = errors.New("dpapi unavailable")
