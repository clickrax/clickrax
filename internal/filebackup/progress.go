package filebackup

import (
	"sync"
	"time"

	"pbs-win-backup/internal/models"
)

// Overall progress ranges (0–100, monotonic).
const (
	progScanEnd     = 5.0
	progArchiveEnd  = 40.0
	progTransferEnd = 92.0
	progManifestEnd = 97.0
)

type progressEmitter struct {
	opts       Options
	kind       string
	startedAt  string
	lastPct    float64
	lastBytes  int64
	bytesTotal int64
	lastTick   time.Time
	mu         sync.Mutex
}

func newProgressEmitter(opts Options, startedAt string) *progressEmitter {
	return &progressEmitter{opts: opts, startedAt: startedAt}
}

func (p *progressEmitter) setKind(kind string) {
	p.mu.Lock()
	p.kind = kind
	p.mu.Unlock()
}

func (p *progressEmitter) setBytesTotal(total int64) {
	p.mu.Lock()
	p.bytesTotal = total
	p.mu.Unlock()
}

func (p *progressEmitter) emit(phase models.ProgressPhase, pct float64, msg string, transferred, reused int64, filesDone, filesTotal int) {
	if p.opts.OnProgress == nil {
		return
	}
	p.mu.Lock()
	if pct < p.lastPct {
		pct = p.lastPct
	} else {
		p.lastPct = pct
	}
	now := time.Now()
	var speed int64
	if !p.lastTick.IsZero() && transferred >= p.lastBytes {
		if dt := now.Sub(p.lastTick).Seconds(); dt > 0 {
			speed = int64(float64(transferred-p.lastBytes) / dt)
		}
	}
	bytesTotal := p.bytesTotal
	var eta int64
	if phase == models.PhaseTransfer && bytesTotal > transferred && speed > 0 {
		eta = int64(float64(bytesTotal-transferred) / float64(speed))
	}
	p.lastBytes = transferred
	p.lastTick = now
	kind := p.kind
	p.mu.Unlock()

	p.opts.OnProgress(models.ProgressEvent{
		JobID:            p.opts.Job.ID,
		JobName:          p.opts.Job.Name,
		Phase:            phase,
		BackupType:       kind,
		Percent:          pct,
		BytesTransferred: transferred,
		BytesReused:      reused,
		BytesTotal:       bytesTotal,
		FilesDone:        filesDone,
		FilesTotal:       filesTotal,
		SpeedBps:         speed,
		ETASec:           eta,
		StartedAt:        p.startedAt,
		Message:          msg,
	})
}

func archivePercent(done, total int) float64 {
	if total <= 0 {
		return progScanEnd + 1
	}
	frac := float64(done) / float64(total)
	return progScanEnd + (progArchiveEnd-progScanEnd)*frac
}

func transferPercent(written, total int64) float64 {
	if total <= 0 {
		return progArchiveEnd + 1
	}
	frac := float64(written) / float64(total)
	return progArchiveEnd + (progTransferEnd-progArchiveEnd)*frac
}
