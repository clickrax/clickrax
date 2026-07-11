package pbsbackup

import "testing"

func TestEffectiveChunkWorkersConfigured(t *testing.T) {
	if got := effectiveChunkWorkers(16); got != 16 {
		t.Fatalf("expected 16, got %d", got)
	}
	if got := effectiveChunkWorkers(64); got != maxChunkWorkers {
		t.Fatalf("expected cap %d, got %d", maxChunkWorkers, got)
	}
}

func TestEffectiveChunkWorkersAuto(t *testing.T) {
	got := effectiveChunkWorkers(0)
	if got < minChunkWorkers || got > maxChunkWorkers {
		t.Fatalf("auto workers out of range: %d", got)
	}
}

func TestSetChunkWorkersSetting(t *testing.T) {
	SetChunkWorkersSetting(20)
	if ChunkWorkers() != 20 {
		t.Fatalf("expected 20, got %d", ChunkWorkers())
	}
	SetChunkWorkersSetting(0)
	if ChunkWorkers() < minChunkWorkers {
		t.Fatalf("expected auto >= %d, got %d", minChunkWorkers, ChunkWorkers())
	}
}
