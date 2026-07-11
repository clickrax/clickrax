package pbsbackup

import (
	"testing"
	"time"
)

func TestEffectiveProcessedBytes_prefersVirtualDuringFastReuse(t *testing.T) {
	s := pbsProgressSample{
		bytesNew:        100,
		bytesReused:     200,
		virtualBytes:    5_000_000,
		fastReuseActive: true,
	}
	if got := effectiveProcessedBytes(s); got != 5_000_000 {
		t.Fatalf("effectiveProcessedBytes = %d, want 5000000", got)
	}
}

func TestComputePBSSpeedAndETA_fastReuseUsesVirtualBytes(t *testing.T) {
	rates := &pbsProgressRates{lastTick: time.Now().Add(-time.Second)}
	speed, eta := computePBSSpeedAndETA(pbsProgressSample{
		virtualBytes:       10_000_000,
		fastReuseActive:    true,
		bytesTotalEstimate: 100_000_000,
		filesDone:          1000,
		filesTotalEstimate: 10_000,
	}, rates, time.Now())
	if speed <= 0 {
		t.Fatalf("speed = %d, want > 0", speed)
	}
	if eta <= 0 {
		t.Fatalf("eta = %d, want > 0", eta)
	}
}

func TestComputePBSSpeedAndETA_fileFallbackETA(t *testing.T) {
	rates := &pbsProgressRates{
		lastTick:  time.Now().Add(-time.Second),
		lastFiles: 100,
	}
	_, eta := computePBSSpeedAndETA(pbsProgressSample{
		fastReuseActive:    true,
		filesDone:          200,
		filesTotalEstimate: 10_000,
	}, rates, time.Now())
	if eta <= 0 {
		t.Fatalf("eta = %d, want file-based estimate", eta)
	}
}

func TestComputePBSTransferPercent_fileBasedDuringFastReuse(t *testing.T) {
	pct := computePBSTransferPercent(pbsProgressSample{
		fastReuseActive:    true,
		filesDone:          50_000,
		filesTotalEstimate: 200_000,
	}, 1, 0)
	want := 25.0 + 50_000.0/200_000.0*50.0
	if pct != want {
		t.Fatalf("pct = %v, want %v", pct, want)
	}
}

func TestComputePBSTransferPercent_usesDoneWhenAboveCache(t *testing.T) {
	pct := computePBSTransferPercent(pbsProgressSample{
		fastReuseActive:    true,
		filesDone:          305_329,
		filesTotalEstimate: 248_507,
	}, 1, 0)
	if pct != 75.0 {
		t.Fatalf("pct = %v, want 75 capped", pct)
	}
}
