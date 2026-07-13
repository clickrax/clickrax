package pbsbackup

import "testing"

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
