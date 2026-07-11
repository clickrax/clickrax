package main

import (
	"strings"
	"time"

	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/schedule"
)

const maxRecentExecutionRecords = 100

const activeCheckpointMaxAge = 3 * time.Minute

func isInProgressPhase(p models.ProgressPhase) bool {
	switch p {
	case models.PhasePreparing, models.PhaseAnalyzing, models.PhaseVSS,
		models.PhaseTransfer, models.PhaseFinalizing, models.PhaseVerify:
		return true
	default:
		return false
	}
}

func isTerminalPhase(p models.ProgressPhase) bool {
	switch p {
	case models.PhaseDone, models.PhaseCancelled, models.PhaseError:
		return true
	default:
		return false
	}
}

func backupPreparingMessage(b *i18n.Bundle, cfg *models.Config, jobID string) string {
	if cfg == nil {
		return b.T("backup.connecting_pbs")
	}
	for _, j := range cfg.Jobs {
		if j.ID != jobID {
			continue
		}
		destID := j.EffectiveDestinationID()
		dest, ok := models.FindDestination(cfg, destID)
		if !ok || dest == nil {
			return b.T("backup.connecting_pbs")
		}
		if dest.IsPBS() {
			return b.T("backup.connecting_pbs")
		}
		destType := strings.ToUpper(strings.TrimSpace(dest.Type))
		if destType == "" {
			destType = "SMB"
		}
		return b.Tf("backup.archive_upload", map[string]string{"type": destType})
	}
	return b.T("backup.connecting_pbs")
}

func (a *App) GetExecutionState() models.ExecutionState {
	b := a.bundle()
	state := models.ExecutionState{
		Active:          []models.ExecutionRun{},
		Upcoming:        []models.ScheduledRunInfo{},
		RecentManual:    []models.JobRunRecord{},
		RecentScheduled: []models.JobRunRecord{},
	}

	now := time.Now()
	a.mu.RLock()
	cfg := a.store.ConfigSnapshot()
	running := a.engine.IsRunning()
	lp := a.lastProgress
	a.mu.RUnlock()

	if cfg != nil {
		upcoming := schedule.ListUpcoming(cfg.Jobs, now, 12)
		state.Upcoming = make([]models.ScheduledRunInfo, len(upcoming))
		for i, u := range upcoming {
			state.Upcoming[i] = u
			state.Upcoming[i].JobName = b.RetranslateJobName(u.JobName)
			state.Upcoming[i].BackupType = i18n.NormalizeBackupType(u.BackupType)
		}
	}

	if queued := backupqueue.ListBestEffort(); len(queued) > 0 {
		for i, q := range queued {
			state.Queued = append(state.Queued, models.QueuedRunInfo{
				JobID:      q.JobID,
				JobName:    b.RetranslateJobName(q.JobName),
				Trigger:    q.Trigger,
				Position:   i + 1,
				EnqueuedAt: q.EnqueuedAt.Format(time.RFC3339),
			})
		}
	}

	seenActive := map[string]struct{}{}
	if running {
		if lp.JobID != "" && isInProgressPhase(lp.Phase) {
			trigger := lp.Trigger
			if trigger == "" {
				trigger = "manual"
			}
			state.Active = append(state.Active, localizeExecutionRun(b, progressToExecution(lp, trigger, true)))
			seenActive[lp.JobID] = struct{}{}
		} else if currentJobID := a.engine.CurrentJobID(); currentJobID != "" {
			if lp.JobID == currentJobID && isTerminalPhase(lp.Phase) {
				// Engine still finishing cleanup; avoid flashing a synthetic 0% active run.
			} else if _, ok := seenActive[currentJobID]; !ok {
				jobName := ""
				if cfg != nil {
					for _, j := range cfg.Jobs {
						if j.ID == currentJobID {
							jobName = j.Name
							break
						}
					}
				}
				trigger := "manual"
				if lp.JobID == currentJobID && lp.Trigger != "" {
					trigger = lp.Trigger
				}
				state.Active = append(state.Active, localizeExecutionRun(b, models.ExecutionRun{
					JobID:     currentJobID,
					JobName:   jobName,
					Trigger:   trigger,
					Phase:     string(models.PhasePreparing),
					Message:   backupPreparingMessage(b, cfg, currentJobID),
					StartedAt: lp.StartedAt,
					CanStop:   true,
				}))
				seenActive[currentJobID] = struct{}{}
			}
		}
	}

	activeCP, _ := pbsbackup.ListActiveCheckpoints(activeCheckpointMaxAge)
	for _, cp := range activeCP {
		if _, ok := seenActive[cp.JobID]; ok {
			continue
		}
		trigger := cp.Trigger
		if trigger == "" {
			trigger = "manual"
		}
		state.Active = append(state.Active, localizeExecutionRun(b, checkpointToExecution(b, cp, trigger, false)))
		seenActive[cp.JobID] = struct{}{}
	}

	cps, _ := pbsbackup.ListInterruptedCheckpoints()
	for _, cp := range cps {
		if _, ok := seenActive[cp.JobID]; ok {
			continue
		}
		trigger := cp.Trigger
		if trigger == "" {
			trigger = "manual"
		}
		state.Interrupted = append(state.Interrupted, localizeExecutionRun(b, checkpointToExecution(b, cp, trigger, true)))
	}

	a.ensureHistoryLoaded()
	a.mu.RLock()
	records := append([]models.JobRunResult(nil), a.history...)
	a.mu.RUnlock()
	for _, r := range records {
		rec := r.ToRecord()
		trigger := rec.Trigger
		if trigger == "" {
			trigger = "manual"
		}
		if trigger == "scheduled" {
			if len(state.RecentScheduled) >= maxRecentExecutionRecords {
				continue
			}
			state.RecentScheduled = append(state.RecentScheduled, localizeJobRecord(b, rec))
		} else {
			if len(state.RecentManual) >= maxRecentExecutionRecords {
				continue
			}
			state.RecentManual = append(state.RecentManual, localizeJobRecord(b, rec))
		}
	}

	return state
}

func progressToExecution(p models.ProgressEvent, trigger string, canStop bool) models.ExecutionRun {
	if p.Trigger != "" {
		trigger = p.Trigger
	}
	return models.ExecutionRun{
		JobID:            p.JobID,
		JobName:          p.JobName,
		Trigger:          trigger,
		Phase:            string(p.Phase),
		BackupType:       p.BackupType,
		Percent:          p.Percent,
		BytesTransferred: p.BytesTransferred,
		BytesReused:      p.BytesReused,
		SpeedBps:         p.SpeedBps,
		ETASec:           p.ETASec,
		StartedAt:        p.StartedAt,
		ChunksNew:        p.ChunksNew,
		ChunksReused:     p.ChunksReused,
		FilesDone:        p.FilesDone,
		FilesTotal:       p.FilesTotal,
		FilesSkipped:     p.FilesSkipped,
		CurrentPath:      p.CurrentPath,
		Message:          p.Message,
		CanStop:          canStop,
	}
}

func checkpointToExecution(b *i18n.Bundle, cp pbsbackup.Checkpoint, trigger string, interrupted bool) models.ExecutionRun {
	started := ""
	if !cp.StartedAt.IsZero() {
		started = cp.StartedAt.Format(time.RFC3339)
	}
	updated := ""
	if !cp.UpdatedAt.IsZero() {
		updated = cp.UpdatedAt.Format(time.RFC3339)
	} else {
		updated = started
	}
	msg := cp.Phase
	if cp.Error != "" {
		msg = cp.Error
	} else if cp.Phase == string(models.PhaseCancelled) {
		msg = b.T("backup.interrupted")
	} else if interrupted {
		msg = b.T("backup.interrupted_incomplete")
	}
	return models.ExecutionRun{
		JobID:            cp.JobID,
		JobName:          cp.JobName,
		Trigger:          trigger,
		Phase:            cp.Phase,
		Percent:          cp.Percent,
		BytesTransferred: cp.BytesTransferred,
		BytesReused:      cp.BytesReused,
		ChunksNew:        int(cp.NewChunks),
		ChunksReused:     int(cp.ReusedChunks),
		FilesDone:        int(cp.FilesDone),
		FilesTotal:       int(cp.FilesTotal),
		FilesSkipped:     int(cp.FilesSkipped),
		Message:          msg,
		StartedAt:        started,
		UpdatedAt:        updated,
		CanStop:          !interrupted,
		CanRetry:         interrupted,
		CanDismiss:       interrupted,
	}
}

func localizeJobRecord(b *i18n.Bundle, rec models.JobRunRecord) models.JobRunRecord {
	rec.JobName = b.RetranslateJobName(rec.JobName)
	rec.Error = b.RetranslateStored(rec.Error)
	rec.Message = b.RetranslateStored(rec.Message)
	rec.BackupType = i18n.NormalizeBackupType(rec.BackupType)
	return rec
}

func localizeExecutionRun(b *i18n.Bundle, run models.ExecutionRun) models.ExecutionRun {
	run.JobName = b.RetranslateJobName(run.JobName)
	run.Message = b.RetranslateStored(run.Message)
	run.BackupType = i18n.NormalizeBackupType(run.BackupType)
	return run
}
