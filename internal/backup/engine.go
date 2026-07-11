package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/backupcancel"
	"pbs-win-backup/internal/backuproot"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/filebackup"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/pbsbackup"
)

type RunParams struct {
	Job              models.BackupJob
	Destination      *models.BackupDestination
	Secret           string
	GlobalExclusions []string
	ForceFull        bool
	BandwidthMbps       int
	NetworkTimeoutSec   int
	Trigger             string // manual, scheduled
	Lang                string
}

type Engine struct {
	mu           sync.Mutex
	running      bool
	stopping     bool
	currentJobID string
	cancel       context.CancelFunc
	onEvent      func(models.ProgressEvent)
}

func NewEngine(onEvent func(models.ProgressEvent)) *Engine {
	return &Engine{onEvent: onEvent}
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) IsStopping() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stopping
}

func (e *Engine) CurrentJobID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentJobID
}

func (e *Engine) emit(ev models.ProgressEvent) {
	if e.onEvent != nil {
		e.onEvent(ev)
	}
}

func (e *Engine) Run(ctx context.Context, params RunParams) (models.JobRunResult, error) {
	b := i18n.New(params.Lang)
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return models.JobRunResult{}, errJobRunning{lang: b.Lang()}
	}
	e.running = true
	e.stopping = false
	e.currentJobID = params.Job.ID
	runCtx, cancel := context.WithCancel(ctx)
	if params.NetworkTimeoutSec > 0 {
		runCtx = netio.WithIdleTimeout(runCtx, params.NetworkTimeoutSec)
	}
	e.cancel = cancel
	e.mu.Unlock()

	if backupcancel.IsRequested(params.Job.ID) {
		e.mu.Lock()
		e.running = false
		e.currentJobID = ""
		e.mu.Unlock()
		return models.JobRunResult{Status: "cancelled", Error: b.T("backup.cancelled")}, context.Canceled
	}

	defer backupcancel.Clear(params.Job.ID)

	pollDone := make(chan struct{})
	defer close(pollDone)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-pollDone:
				return
			case <-ticker.C:
				if backupcancel.IsRequested(params.Job.ID) {
					backupcancel.Clear(params.Job.ID)
					e.Stop()
					return
				}
			}
		}
	}()

	i18n.SetActive(b)
	defer i18n.SetActive(nil)

	started := time.Now()
	startedAtRFC := started.UTC().Format(time.RFC3339)
	trigger := params.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	emitProgress := func(ev models.ProgressEvent) {
		if ev.StartedAt == "" {
			ev.StartedAt = startedAtRFC
		}
		if ev.Trigger == "" {
			ev.Trigger = trigger
		}
		e.emit(ev)
	}
	result := models.JobRunResult{
		JobID:     params.Job.ID,
		JobName:   params.Job.Name,
		StartedAt: started,
		Trigger:   trigger,
	}

	defer func() {
		e.mu.Lock()
		e.running = false
		e.stopping = false
		e.currentJobID = ""
		e.cancel = nil
		e.mu.Unlock()
		result.FinishedAt = time.Now()
		result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
	}()

	job := params.Job

	if len(job.Sources) == 0 {
		result.Status = "error"
		result.Error = b.T("backup.no_sources")
		return result, b.E("backup.no_sources")
	}

	if params.Destination == nil {
		return result, b.E("backup.dest_not_configured")
	}
	dest := *params.Destination

	if job.EncryptionEnabled && dest.IsPBS() {
		if _, err := credential.GetPassphrase(params.Job.ID); err != nil {
			result.Status = "error"
			result.Error = b.T("backup.encryption_no_passphrase")
			emitProgress(models.ProgressEvent{
				JobID: job.ID, JobName: job.Name,
				Phase: models.PhaseError, Message: result.Error,
			})
			return result, b.E("backup.encryption_no_passphrase")
		}
		result.Status = "error"
		result.Error = b.T("backup.encryption_ece_unsupported")
		emitProgress(models.ProgressEvent{
			JobID: job.ID, JobName: job.Name,
			Phase: models.PhaseError, Message: result.Error,
		})
		return result, b.E("backup.encryption_ece_unsupported")
	}

	_ = backuplock.ClearStale()
	lock, err := backuplock.Acquire()
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}
	defer lock.Release()

	switch dest.NormalizedType() {
	case models.DestSMB, models.DestFTP:
		emitProgress(models.ProgressEvent{
			JobID: job.ID, JobName: job.Name,
			Phase: models.PhasePreparing, Percent: 0,
			Message: b.Tf("backup.archive_upload", map[string]string{"type": strings.ToUpper(dest.NormalizedType())}),
		})
		stats, err := filebackup.Run(runCtx, filebackup.Options{
			Destination:      dest,
			Password:         params.Secret,
			Job:              job,
			GlobalExclusions: params.GlobalExclusions,
			Hostname:         Hostname(),
			ForceFull:        params.ForceFull,
			OnProgress:       emitProgress,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				result.Status = "cancelled"
				result.Error = b.T("backup.cancelled")
				emitProgress(models.ProgressEvent{
					JobID: job.ID, JobName: job.Name,
					Phase: models.PhaseCancelled, Percent: 0,
					Message: result.Error,
				})
				return result, err
			}
			result.Status = "error"
			result.Error = err.Error()
			eventlog.Error(fmt.Sprintf("бэкап %s: %s", job.Name, err.Error()))
			emitProgress(models.ProgressEvent{
				JobID: job.ID, JobName: job.Name,
				Phase: models.PhaseError, Percent: 0,
				Message: err.Error(),
			})
			return result, err
		}
		result.BackupType = stats.BackupKind
		if result.BackupType == "" {
			result.BackupType = "incremental"
		}
		result.BytesTransferred = stats.BytesTransferred.Load()
		result.BytesReused = stats.BytesSkipped.Load()
		result.FilesTotal = int(stats.FilesTotal.Load())
		result.FilesSkipped = int(stats.FilesSkipped.Load())
		result.Status = "ok"
		if result.FilesSkipped > 0 {
			result.Status = "warning"
		}
		result.Snapshot = stats.RemotePath
		emitProgress(models.ProgressEvent{
			JobID:            job.ID,
			JobName:          job.Name,
			Phase:            models.PhaseDone,
			Percent:          100,
			BytesTransferred: result.BytesTransferred,
			BytesReused:      result.BytesReused,
			BackupType:       result.BackupType,
			Message: b.Tf("backup.completed_type_dest", map[string]string{
				"type": result.BackupType,
				"dest": stats.RemotePath,
			}),
		})
		eventlog.Info(fmt.Sprintf("бэкап %s завершён (%s, файл %s): передано %d Б, пропущено %d Б",
			job.Name, result.BackupType, stats.RemotePath, result.BytesTransferred, result.BytesReused))
		return result, nil
	}

	if params.Secret == "" {
		return result, b.E("backup.secret_not_found")
	}
	server := dest.ToPBSServer()

	emitProgress(models.ProgressEvent{
		JobID: job.ID, JobName: job.Name,
		Phase: models.PhasePreparing, Percent: 0,
		Message: b.T("backup.connecting_pbs"),
	})

	var bytesTotalEstimate int64
	if backupRoot, cleanup, resolveErr := backuproot.Resolve(job.Sources); resolveErr == nil {
		if scan, scanErr := ScanPath(backupRoot, params.GlobalExclusions, job.Exclusions); scanErr == nil {
			bytesTotalEstimate = scan.Bytes
		}
		cleanup()
	}

	stats, err := pbsbackup.Run(runCtx, pbsbackup.Options{
		Server:             server,
		Secret:             params.Secret,
		Job:                job,
		GlobalExclusions:   params.GlobalExclusions,
		BytesTotalEstimate: bytesTotalEstimate,
		ForceFull:          params.ForceFull,
		BandwidthMbps:      params.BandwidthMbps,
		Trigger:            trigger,
		OnProgress: func(ev models.ProgressEvent) {
			emitProgress(ev)
		},
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			result.Status = "cancelled"
			result.Error = b.T("backup.cancelled")
			emitProgress(models.ProgressEvent{
				JobID: job.ID, JobName: job.Name,
				Phase: models.PhaseCancelled, Percent: 0,
				Message: result.Error,
			})
			return result, err
		}
		result.Status = "error"
		result.Error = err.Error()
		eventlog.Error(fmt.Sprintf("бэкап %s: %s", job.Name, err.Error()))
		emitProgress(models.ProgressEvent{
			JobID: job.ID, JobName: job.Name,
			Phase: models.PhaseError, Percent: 0,
			Message: err.Error(),
		})
		return result, err
	}
	result.BackupType = "incremental"
	if params.ForceFull || stats.ReusedChunks.Load() == 0 {
		result.BackupType = "full"
	}
	result.BytesTransferred = stats.BytesNew.Load()
	result.BytesReused = stats.BytesReused.Load()
	result.FilesTotal = int(stats.FilesTotal.Load())
	result.FilesSkipped = int(stats.FilesSkipped.Load())
	result.Status = "ok"
	if result.FilesSkipped > 0 {
		result.Status = "warning"
	}
	if stats.Warning != "" {
		result.Status = "warning"
		if result.Message == "" {
			result.Message = stats.Warning
		}
	}
	if stats.BackupTimeUnix > 0 {
		result.Snapshot = time.Unix(stats.BackupTimeUnix, 0).UTC().Format(time.RFC3339)
	} else {
		result.Snapshot = time.Now().UTC().Format(time.RFC3339)
	}
	emitProgress(models.ProgressEvent{
		JobID:            job.ID,
		JobName:          job.Name,
		Phase:            models.PhaseDone,
		Percent:          100,
		BytesTransferred: result.BytesTransferred,
		BytesReused:      result.BytesReused,
		BackupType:       result.BackupType,
		Message:          b.Tf("backup.completed_type", map[string]string{"type": result.BackupType}),
	})
	eventlog.Info(fmt.Sprintf("бэкап %s завершён (%s): передано %d Б, переиспользовано %d Б",
		job.Name, result.BackupType, result.BytesTransferred, result.BytesReused))
	return result, nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	e.stopping = true
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()
}

var ErrJobAlreadyRunning = errJobRunning{}

type errJobRunning struct {
	lang string
}

func (e errJobRunning) Error() string {
	return i18n.New(e.lang).T("backup.job_running")
}

func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "windows-host"
	}
	return strings.TrimSpace(h)
}
