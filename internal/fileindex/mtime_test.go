package fileindex

import (
	"testing"
	"time"
)

func TestMtimeMatchesSecondPrecision(t *testing.T) {
	sec := time.Unix(1710000000, 123456789)
	stored := sec.Unix() * 1e9
	if !MtimeMatches(int64(stored), sec.UnixNano()) {
		t.Fatal("second-precision mtime should match")
	}
	if MtimeMatches(int64(stored), sec.Add(time.Second).UnixNano()) {
		t.Fatal("different seconds should not match")
	}
}
