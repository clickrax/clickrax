//go:build windows

package backup

import (
	"runtime"
	"testing"

	"pbs-win-backup/internal/config"
)

func TestScanPathVolumeUsesFastEstimate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	res, err := ScanPath(`C:\`, config.DefaultExclusions(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Approx {
		t.Fatal("expected approx volume estimate")
	}
	if res.Bytes <= 0 {
		t.Fatalf("expected positive used bytes, got %d", res.Bytes)
	}
}
