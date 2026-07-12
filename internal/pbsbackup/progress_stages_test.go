package pbsbackup

import "testing"

func TestFinalizeStagePercent_coversAllStages(t *testing.T) {
	for id := range finalizeStageI18n {
		if _, ok := finalizeStagePercentMap[id]; !ok {
			t.Fatalf("missing percent for stage %q", id)
		}
	}
}

func TestFinalizeStagePercent_inFinalizingRange(t *testing.T) {
	for id, want := range finalizeStagePercentMap {
		if want <= 75 || want >= 98 {
			t.Fatalf("stage %q pct %v outside 76-97", id, want)
		}
	}
}

func TestIsFinalizeStage(t *testing.T) {
	if !isFinalizeStage(stageFinalizeSavePxarIdx) {
		t.Fatal("expected finalize stage")
	}
	if isFinalizeStage("scan_files") {
		t.Fatal("unexpected finalize stage")
	}
}
