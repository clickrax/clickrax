//go:build windows

package service

import (
	"context"
	"os"
	"time"

	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/backupcancel"
	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	appeventlog "pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/pbsbackup"
	schedpkg "pbs-win-backup/internal/schedule"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "PbsWinBackup"

type handler struct {
	engine      *backup.Engine
	shutdownCtx context.Context
}

func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	elog, _ := eventlog.Open(serviceName)
	if elog != nil {
		_ = elog.Info(1, "служба "+branding.Name+" запущена")
		defer elog.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	h.shutdownCtx = ctx
	paths.EnsureSharedDataAccess()
	_ = backuplock.ClearStale()
	ClearStaleScheduleClaims()
	_ = backupqueue.ReconcileInflight()
	backupcancel.ReapStale(0)
	h.processBackupQueue()

	if cfg, err := config.Load(); err == nil {
		ids := make([]string, 0, len(cfg.Destinations))
		for _, d := range cfg.Destinations {
			ids = append(ids, d.ID)
		}
		credential.MigrateSecrets(ids)
		credential.MigrateSMTPPassword()
		jobIDs := make([]string, 0, len(cfg.Jobs))
		for _, j := range cfg.Jobs {
			if j.EncryptionEnabled {
				jobIDs = append(jobIDs, j.ID)
			}
		}
		credential.MigratePassphrases(jobIDs)
	}

	go runAlignedScheduleLoop(ctx, func(now time.Time) {
		h.checkSchedule(ctx, elog, now)
	})

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			if h.engine != nil && h.engine.IsRunning() {
				appeventlog.Info("остановка службы: прерывание активного бэкапа…")
				h.engine.Stop()
				deadline := time.Now().Add(60 * time.Second)
				for h.engine.IsRunning() && time.Now().Before(deadline) {
					time.Sleep(200 * time.Millisecond)
				}
			}
			_ = backuplock.ForceClearOwn()
			cancel()
			return false, 0
		}
	}
	return false, 0
}

func (h *handler) checkSchedule(ctx context.Context, elog *eventlog.Log, now time.Time) {
	cfg, err := config.Load()
	if err != nil {
		appeventlog.Error("расписание: не удалось загрузить config: " + err.Error())
		return
	}
	pbsbackup.SetChunkWorkersSetting(cfg.Settings.ChunkWorkers)
	h.processBackupQueue()
	for _, job := range cfg.Jobs {
		if !job.Schedule.Enabled {
			continue
		}
		if !MatchesWindow(job, now) {
			continue
		}
		if SlotAlreadySucceeded(job, now) {
			continue
		}
		item := backupqueue.ItemFromJob(job, schedpkg.ShouldForceFull(job.Schedule, now), now)
		if contained, err := backupqueue.Contains(item); err == nil && contained {
			continue
		}
		if !TryClaimSlot(job, now) {
			continue
		}
		if SlotAlreadySucceeded(job, now) {
			ReleaseSlotClaim(job, now)
			continue
		}
		if !BeginScheduledRun(job.ID, item.SlotKey) {
			ReleaseSlotClaim(job, now)
			continue
		}
		if err := h.submitScheduledBackup(job, now); err != nil {
			EndScheduledRun(job.ID, item.SlotKey)
			ReleaseSlotClaim(job, now)
			appeventlog.Error("расписание " + job.Name + " (" + schedpkg.DescribeRun(job.Schedule, now) + "): " + err.Error())
		}
	}
}

// RunService starts Windows service main loop.
func RunService(isDebug bool) error {
	h := &handler{engine: backup.NewEngine(nil)}
	if isDebug {
		return debug.Run(serviceName, h)
	}
	return svc.Run(serviceName, h)
}

// IsWindowsServiceProcess returns true if launched with --service flag.
func IsWindowsServiceProcess() bool {
	for _, a := range os.Args[1:] {
		if a == "--service" {
			return true
		}
	}
	return false
}
