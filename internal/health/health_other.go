//go:build !windows

package health

import "pbs-win-backup/internal/models"

func Run(cfg *models.Config) Report {
	return Report{OK: true, Checks: []Check{{Name: "platform", OK: true, Message: "health check only on Windows"}}}
}
