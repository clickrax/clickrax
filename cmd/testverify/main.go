package main

import (
	"fmt"
	"os"
	"time"

	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbs"
	"pbs-win-backup/internal/pbsbackup"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
	for _, job := range cfg.Jobs {
		server := findServer(cfg, job.ServerID)
		if server == nil {
			continue
		}
		secret, err := credential.GetSecret(server.ID)
		if err != nil {
			fmt.Println("secret:", err)
			continue
		}
		snaps, err := pbsbackup.ListSnapshots(*server, secret)
		if err != nil {
			fmt.Println("snapshots:", err)
			continue
		}
		var latest int64
		for _, s := range snaps {
			if s.Backup == job.BackupID && s.BackupTime > latest {
				latest = s.BackupTime
			}
		}
		if latest == 0 {
			fmt.Printf("job %s: no snapshots\n", job.Name)
			continue
		}
		fmt.Printf("=== job %s backup-time=%d ===\n", job.Name, latest)
		client := pbs.NewClient(*server, secret)
		upid, err := client.StartSnapshotVerify("host", job.BackupID, latest)
		if err != nil {
			fmt.Println("start verify:", err)
			continue
		}
		fmt.Println("upid:", upid)
		err = client.WaitTaskDuration(2*time.Minute, upid)
		fmt.Println("wait:", err)
	}
}

func findServer(cfg *models.Config, id string) *models.PBSServer {
	for i := range cfg.Servers {
		if cfg.Servers[i].ID == id {
			return &cfg.Servers[i]
		}
	}
	return nil
}
