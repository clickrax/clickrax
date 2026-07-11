package service

type StatusInfo struct {
	Installed     bool
	Running       bool
	PendingDelete bool
	State         string
	Message       string
}
