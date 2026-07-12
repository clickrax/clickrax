package pbsbackup

import "pbs-win-backup/internal/i18n"

const (
	stageFinalizePxarEOF       = "finalize_pxar_eof"
	stageFinalizePcatEOF       = "finalize_pcat_eof"
	stageFinalizeUploadMeta    = "finalize_upload_meta"
	stageFinalizeUploadPxarIdx = "finalize_upload_pxar_idx"
	stageFinalizeManifest      = "finalize_manifest"
	stageFinalizeFinish        = "finalize_finish"
	stageFinalizeSaveFileIdx   = "finalize_save_file_idx"
	stageFinalizeSavePxarIdx   = "finalize_save_pxar_idx"
	stageFinalizeChunkIdx      = "finalize_chunk_idx"
)

var finalizeStageI18n = map[string]string{
	stageFinalizePxarEOF:       "pbs.finalize_pxar_eof",
	stageFinalizePcatEOF:       "pbs.finalize_pcat_eof",
	stageFinalizeUploadMeta:    "pbs.finalize_upload_meta",
	stageFinalizeUploadPxarIdx: "pbs.finalize_upload_pxar_idx",
	stageFinalizeManifest:      "pbs.finalize_manifest",
	stageFinalizeFinish:        "pbs.finalize_finish",
	stageFinalizeSaveFileIdx:   "pbs.finalize_save_file_idx",
	stageFinalizeSavePxarIdx:   "pbs.finalize_save_pxar_idx",
	stageFinalizeChunkIdx:      "pbs.finalize_chunk_idx",
}

var finalizeStagePercentMap = map[string]float64{
	stageFinalizePxarEOF:       76,
	stageFinalizePcatEOF:       79,
	stageFinalizeUploadMeta:    82,
	stageFinalizeUploadPxarIdx: 85,
	stageFinalizeManifest:      88,
	stageFinalizeFinish:        91,
	stageFinalizeSaveFileIdx:   94,
	stageFinalizeSavePxarIdx:   96,
	stageFinalizeChunkIdx:      97,
}

func finalizeStageMessage(id string, params map[string]string) (string, bool) {
	key, ok := finalizeStageI18n[id]
	if !ok {
		return "", false
	}
	if params == nil {
		params = map[string]string{}
	}
	return i18n.L(key, params), true
}

func finalizeStagePercent(id string) (float64, bool) {
	pct, ok := finalizeStagePercentMap[id]
	return pct, ok
}

func isFinalizeStage(id string) bool {
	_, ok := finalizeStagePercentMap[id]
	return ok
}
