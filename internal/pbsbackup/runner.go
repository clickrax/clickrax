package pbsbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pbs-win-backup/internal/backuproot"
	"pbs-win-backup/internal/chunkindex"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
)

// Stats updated during PBS backup (incremental chunk counters).
type Stats struct {
	NewChunks      atomic.Uint64
	ReusedChunks   atomic.Uint64
	BytesNew         atomic.Int64
	BytesReused      atomic.Int64
	FilesTotal           atomic.Int64
	FilesSkipped         atomic.Int64
	VirtualBytesProcessed atomic.Int64
	EstimatedFilesTotal  atomic.Int64
	BackupTimeUnix       int64
	Warning          string
	fastReuseActive  atomic.Bool
	stage            atomic.Value // string: current sub-step message for UI
	stageID          atomic.Value // string: internal stage key for progress mapping
	progressEmit     func(id string, params map[string]string)
}

func (s *Stats) bindProgressEmit(fn func(id string, params map[string]string)) {
	if s == nil {
		return
	}
	s.progressEmit = fn
}

func (s *Stats) SetFastReuseActive(v bool) {
	if s == nil {
		return
	}
	s.fastReuseActive.Store(v)
}

func (s *Stats) FastReuseActive() bool {
	if s == nil {
		return false
	}
	return s.fastReuseActive.Load()
}

func (s *Stats) SetStage(msg string) {
	if s == nil {
		return
	}
	s.stageID.Store("")
	s.stage.Store(msg)
}

func (s *Stats) SetStageID(id string, params map[string]string) {
	if s == nil || id == "" {
		return
	}
	msg, ok := finalizeStageMessage(id, params)
	if !ok {
		return
	}
	s.stageID.Store(id)
	s.stage.Store(msg)
	if s.progressEmit != nil {
		s.progressEmit(id, params)
	}
}

func (s *Stats) StageID() string {
	if s == nil {
		return ""
	}
	v := s.stageID.Load()
	if v == nil {
		return ""
	}
	id, _ := v.(string)
	return id
}

func (s *Stats) Stage() string {
	if s == nil {
		return ""
	}
	v := s.stage.Load()
	if v == nil {
		return ""
	}
	msg, _ := v.(string)
	return msg
}

type Options struct {
	Server             models.PBSServer
	Secret             string
	Job                models.BackupJob
	GlobalExclusions   []string
	BytesTotalEstimate int64
	ForceFull          bool
	BandwidthMbps      int
	Trigger            string
	OnProgress         func(models.ProgressEvent)
}

// Run executes a real directory backup to PBS using proxmoxbackupclient_go libraries.
func Run(ctx context.Context, opts Options) (*Stats, error) {
	var stats Stats
	if len(opts.Job.Sources) == 0 {
		return nil, i18n.E("job.no_sources", nil)
	}

	backupRoot, cleanup, err := backuproot.Resolve(opts.Job.Sources)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	bytesTotalEstimate := opts.BytesTotalEstimate

	backupID := opts.Job.BackupID
	if backupID == "" {
		backupID = hostname()
	}

	started := time.Now()
	var progressMu sync.Mutex
	var progressRates pbsProgressRates

	trigger := opts.Trigger
	if trigger == "" {
		trigger = "manual"
	}

	emit := func(phase models.ProgressPhase, pct float64, msg string, bytesTotal int64) {
		if opts.OnProgress == nil {
			return
		}
		newC := stats.NewChunks.Load()
		reuseC := stats.ReusedChunks.Load()
		bNew := stats.BytesNew.Load()
		bReuse := stats.BytesReused.Load()
		sample := pbsProgressSample{
			bytesNew:           bNew,
			bytesReused:        bReuse,
			virtualBytes:       stats.VirtualBytesProcessed.Load(),
			filesDone:          stats.FilesTotal.Load(),
			filesTotalEstimate: stats.EstimatedFilesTotal.Load(),
			fastReuseActive:    stats.FastReuseActive(),
			bytesTotalEstimate: bytesTotal,
		}
		now := time.Now()
		progressMu.Lock()
		speed, eta := computePBSSpeedAndETA(sample, &progressRates, now)
		progressMu.Unlock()
		filesDone := int(sample.filesDone)
		filesTotal := int(sample.filesTotalEstimate)
		filesSkipped := int(stats.FilesSkipped.Load())
		filesChanged := filesDone - filesSkipped
		if filesChanged < 0 {
			filesChanged = 0
		}
		opts.OnProgress(models.ProgressEvent{
			JobID:            opts.Job.ID,
			JobName:          opts.Job.Name,
			Phase:            phase,
			BackupType:       backupTypeLabel(newC, reuseC, opts.ForceFull),
			Percent:          pct,
			BytesTransferred: bNew,
			BytesReused:      bReuse,
			BytesTotal:       bytesTotal,
			ChunksNew:        int(newC),
			ChunksReused:     int(reuseC),
			ChunksTotal:      int(newC + reuseC),
			SpeedBps:         speed,
			ETASec:           eta,
			FilesDone:        filesDone,
			FilesTotal:       filesTotal,
			FilesSkipped:     filesSkipped,
			FilesChanged:     filesChanged,
			Message:          msg,
		})
	}

	saveCP := func(phase models.ProgressPhase, pct float64, errMsg string) {
		cp := Checkpoint{
			JobID:            opts.Job.ID,
			JobName:          opts.Job.Name,
			Phase:            string(phase),
			Trigger:          trigger,
			NewChunks:        stats.NewChunks.Load(),
			ReusedChunks:     stats.ReusedChunks.Load(),
			BytesTransferred: stats.BytesNew.Load(),
			BytesReused:      stats.BytesReused.Load(),
			FilesDone:        stats.FilesTotal.Load(),
			FilesTotal:       stats.EstimatedFilesTotal.Load(),
			FilesSkipped:     stats.FilesSkipped.Load(),
			Percent:          pct,
			StartedAt:        started,
			Error:            errMsg,
		}
		if bt := stats.BackupTimeUnix; bt > 0 {
			cp.BackupTime = bt
		}
		_ = SaveCheckpoint(cp)
	}

	_ = SaveCheckpoint(Checkpoint{
		JobID:     opts.Job.ID,
		JobName:   opts.Job.Name,
		Phase:     string(models.PhasePreparing),
		Trigger:   trigger,
		Percent:   2,
		StartedAt: started,
	})

	emit(models.PhasePreparing, 2, i18n.L("pbs.connecting", nil), bytesTotalEstimate)
	if bytesTotalEstimate > 0 {
		vol := fmt.Sprintf("%.0f MB", float64(bytesTotalEstimate)/(1024*1024))
		if bytesTotalEstimate >= 1024*1024*1024 {
			vol = fmt.Sprintf("%.1f GB", float64(bytesTotalEstimate)/(1024*1024*1024))
		}
		emit(models.PhasePreparing, 4, i18n.L("pbs.volume_estimate", map[string]string{"vol": vol}), bytesTotalEstimate)
	}

	client := newPBSClient(opts.Server, opts.Secret, backupID)

	stats.bindProgressEmit(func(id string, params map[string]string) {
		pct, ok := finalizeStagePercent(id)
		if !ok {
			return
		}
		msg, ok := finalizeStageMessage(id, params)
		if !ok {
			return
		}
		emit(models.PhaseFinalizing, pct, msg, bytesTotalEstimate)
		saveCP(models.PhaseFinalizing, pct, "")
	})

	progressDone := make(chan struct{})
	var progressWG sync.WaitGroup
	var stopProgressOnce sync.Once
	stopProgress := func() {
		stopProgressOnce.Do(func() {
			close(progressDone)
			progressWG.Wait()
		})
	}
	defer stopProgress()
	progressWG.Add(1)
	go func() {
		defer progressWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-progressDone:
				return
			case <-ticker.C:
				newC := stats.NewChunks.Load()
				reuseC := stats.ReusedChunks.Load()
				files := stats.FilesTotal.Load()
				estFiles := stats.EstimatedFilesTotal.Load()
				stageID := stats.StageID()
				stage := stats.Stage()
				sample := pbsProgressSample{
					bytesNew:           stats.BytesNew.Load(),
					bytesReused:        stats.BytesReused.Load(),
					virtualBytes:       stats.VirtualBytesProcessed.Load(),
					filesDone:          files,
					filesTotalEstimate: estFiles,
					fastReuseActive:    stats.FastReuseActive(),
					bytesTotalEstimate: bytesTotalEstimate,
				}

				phase := models.PhaseAnalyzing
				pct := 10.0
				msg := stage
				if msg == "" {
					msg = i18n.L("pbs.preparing", nil)
				}

				if pctFin, ok := finalizeStagePercent(stageID); ok {
					phase = models.PhaseFinalizing
					pct = pctFin
				} else if newC+reuseC > 0 || effectiveProcessedBytes(sample) > 0 || (stats.FastReuseActive() && files > 0) {
					phase = models.PhaseTransfer
					skipped := stats.FilesSkipped.Load()
					if skipped > 0 && stats.FastReuseActive() {
						msg = i18n.L("pbs.fast_inc_skipped", map[string]string{
							"count": fmt.Sprintf("%d", skipped),
							"n":     fmt.Sprintf("%d", newC),
							"max":   fmt.Sprintf("%d", reuseC),
						})
					} else {
						msg = i18n.L("pbs.transfer_chunks", map[string]string{
							"n":   fmt.Sprintf("%d", newC),
							"max": fmt.Sprintf("%d", reuseC),
						})
					}
					pct = computePBSTransferPercent(sample, newC, reuseC)
				} else if files > 0 {
					msg = i18n.L("pbs.scanning_files", map[string]string{"n": fmt.Sprintf("%d", files)})
					pct = 15.0
					if bytesTotalEstimate > 0 {
						pct = 12.0
					}
				} else if stageIsFastInc(stage) {
					pct = 10.0
					msg = stage
				} else if stageIsIndex(stage) {
					pct = 12.0
				} else if strings.Contains(stage, "VSS") {
					pct = 8.0
				}

				emit(phase, pct, msg, bytesTotalEstimate)
				saveCP(phase, pct, "")
			}
		}
	}()

	emit(models.PhaseVSS, 8, i18n.L("pbs.vss_prep", nil), 0)
	stats.SetStage(i18n.L("pbs.vss_prep", nil))
	skipAccess := opts.Job.SkipAccessErrors
	known, err := runDirectoryBackup(ctx, client, opts.Server, opts.Secret, backupRoot, opts.Job.VSSEnabled, &stats, opts.Job.ID, opts.ForceFull, opts.BandwidthMbps, opts.GlobalExclusions, opts.Job.Exclusions, skipAccess)

	if err != nil {
		phase := models.PhaseError
		msg := err.Error()
		if errors.Is(err, context.Canceled) {
			phase = models.PhaseCancelled
			msg = i18n.L("backup.interrupted", nil)
		}
		pct := 0.0
		if bytesTotalEstimate > 0 {
			sample := pbsProgressSample{
				bytesNew:           stats.BytesNew.Load(),
				bytesReused:        stats.BytesReused.Load(),
				virtualBytes:       stats.VirtualBytesProcessed.Load(),
				filesDone:          stats.FilesTotal.Load(),
				filesTotalEstimate: stats.EstimatedFilesTotal.Load(),
				fastReuseActive:    stats.FastReuseActive(),
				bytesTotalEstimate: bytesTotalEstimate,
			}
			if processed := effectiveProcessedBytes(sample); processed > 0 {
				pct = computePBSTransferPercent(sample, stats.NewChunks.Load(), stats.ReusedChunks.Load())
			}
		}
		saveCP(phase, pct, msg)
		return nil, err
	}

	stopProgress()
	stats.bindProgressEmit(nil)

	emit(models.PhaseFinalizing, 98, i18n.L("pbs.finalizing", nil), 0)
	backupTime := client.Manifest.BackupTime
	if backupTime <= 0 {
		backupTime = time.Now().Unix()
	}
	snapshotTime := time.Unix(backupTime, 0).UTC().Format(time.RFC3339)
	stats.BackupTimeUnix = backupTime

	if verifyAfterBackupEnabled(opts.Job) {
		emit(models.PhaseVerify, 99, i18n.L("pbs.verify", nil), 0)
		bytesProcessed := stats.BytesNew.Load() + stats.BytesReused.Load()
		if verr := VerifyAfterBackup(ctx, opts.Server, opts.Secret, backupID, backupTime, bytesProcessed); verr != nil {
			if VerifyTimeout(verr) {
				stats.Warning = i18n.L("pbs.verify_timeout", nil)
				emit(models.PhaseVerify, 99, stats.Warning, bytesProcessed)
			} else {
				saveCP(models.PhaseError, 99, verr.Error())
				return nil, verr
			}
		}
	}

	if known != nil {
		stats.SetStageID(stageFinalizeChunkIdx, map[string]string{
			"n": fmt.Sprintf("%d", known.Len()),
		})
		if err := chunkindex.Save(opts.Job.ID, known.ToHashmap(), snapshotTime); err != nil {
			msg := i18n.Ewrap("pbs.chunk_index_save", nil, err).Error()
			eventlog.Error(msg)
			if stats.Warning == "" {
				stats.Warning = i18n.L("pbs.chunk_index_save_warn", map[string]string{"err": err.Error()})
			}
		}
	}

	_ = ClearCheckpoint(opts.Job.ID)
	emit(models.PhaseDone, 100, i18n.L("pbs.done_ok", nil), stats.BytesNew.Load())
	saveCP(models.PhaseDone, 100, "")
	return &stats, nil
}

func stageIsFastInc(stage string) bool {
	s := strings.ToLower(stage)
	return strings.Contains(s, "fast incremental") ||
		strings.Contains(s, "быстрого инкремента") ||
		strings.Contains(s, "подготовка быстрого") ||
		strings.Contains(s, "quick incremental")
}

func stageIsIndex(stage string) bool {
	s := strings.ToLower(stage)
	return strings.Contains(s, "index") || strings.Contains(s, "индекс")
}

func backupTypeLabel(newC, reuseC uint64, forceFull bool) string {
	if forceFull || (reuseC == 0 && newC > 0) {
		return "full"
	}
	if reuseC > 0 && newC > 0 {
		return "incremental"
	}
	return "incremental"
}

func hostname() string {
	return backupHostname()
}
