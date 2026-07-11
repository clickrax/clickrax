package fileindex

import "testing"

func TestIsIncrementalArchiveName(t *testing.T) {
	if !IsIncrementalArchiveName("host_20260102-100000_incr.zip") {
		t.Fatal("expected incremental name")
	}
	if IsIncrementalArchiveName("host_20260102-100000.zip") {
		t.Fatal("full archive should not match")
	}
}
