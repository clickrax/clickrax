package filebackup

import (
	"testing"

	"pbs-win-backup/internal/models"
)

func TestArchivePercent_monotonic(t *testing.T) {
	prev := 0.0
	for done := 0; done <= 100; done++ {
		pct := archivePercent(done, 100)
		if pct < prev {
			t.Fatalf("done=%d: %f < prev %f", done, pct, prev)
		}
		prev = pct
	}
	if archivePercent(100, 100) != progArchiveEnd {
		t.Fatalf("want %f at end, got %f", progArchiveEnd, archivePercent(100, 100))
	}
}

func TestTransferPercent_monotonic(t *testing.T) {
	const total int64 = 1000
	prev := 0.0
	for written := int64(0); written <= total; written += 50 {
		pct := transferPercent(written, total)
		if pct < prev {
			t.Fatalf("written=%d: %f < prev %f", written, pct, prev)
		}
		prev = pct
	}
}

func TestProgressEmitter_monotonic(t *testing.T) {
	var last float64
	pe := newProgressEmitter(Options{}, "2026-01-01T00:00:00Z")
	pe.opts.OnProgress = func(ev models.ProgressEvent) {
		if ev.Percent < last {
			t.Fatalf("percent dropped %f -> %f", last, ev.Percent)
		}
		last = ev.Percent
	}
	pe.emit(models.PhaseAnalyzing, 50, "a", 0, 0, 1, 10)
	pe.emit(models.PhaseAnalyzing, 9, "b", 0, 0, 2, 10) // must stay >= 50
	if last < 50 {
		t.Fatalf("expected monotonic hold at 50, got %f", last)
	}
}
