package pbsbackup

import (
	"testing"
)

func TestDownloadChunksParallelPreservesIndices(t *testing.T) {
	records := []didxRecord{
		{offset: 3, digest: "aaa"},
		{offset: 6, digest: "bbb"},
		{offset: 9, digest: "ccc"},
	}
	got, err := downloadChunksParallel(t.Context(), func(digest string) ([]byte, error) {
		switch digest {
		case "aaa":
			return []byte("one"), nil
		case "bbb":
			return []byte("tw"), nil
		case "ccc":
			return []byte("o"), nil
		default:
			t.Fatalf("unexpected digest %q", digest)
			return nil, nil
		}
	}, records, []int{0, 2}, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got[0]) != "one" || string(got[2]) != "o" {
		t.Fatalf("unexpected chunks: %#v", got)
	}
}
