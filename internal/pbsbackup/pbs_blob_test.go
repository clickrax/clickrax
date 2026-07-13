package pbsbackup

import "testing"

func TestValidPBSBlobName(t *testing.T) {
	if err := validPBSBlobName("backup.winmeta.blob"); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if err := validPBSBlobName("backup.winmeta.json"); err == nil {
		t.Fatal("json suffix must be rejected")
	}
}

func TestBlobFitsPBSLimit(t *testing.T) {
	if !blobFitsPBSLimit(pbsMaxBlobPayloadBytes) {
		t.Fatal("max payload should fit")
	}
	if blobFitsPBSLimit(pbsMaxBlobPayloadBytes + 1) {
		t.Fatal("payload over limit must not fit")
	}
	if pbsEncodedBlobSize(pbsMaxBlobPayloadBytes) != pbsMaxEncodedBlobBytes {
		t.Fatalf("encoded size = %d, want %d", pbsEncodedBlobSize(pbsMaxBlobPayloadBytes), pbsMaxEncodedBlobBytes)
	}
}
