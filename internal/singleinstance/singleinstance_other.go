//go:build !windows

package singleinstance

func Acquire() error  { return nil }
func Release()          {}
