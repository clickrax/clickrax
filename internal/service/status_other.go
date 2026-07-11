//go:build !windows

package service

func QueryStatus() StatusInfo {
	return StatusInfo{Message: "только Windows"}
}
