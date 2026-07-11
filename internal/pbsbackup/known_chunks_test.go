package pbsbackup

import "testing"

func TestParseKnownFromPreviousDigestLookup(t *testing.T) {
	body := make([]byte, 4096+80)
	copy(body, didxMagic)
	for i := 0; i < 2; i++ {
		off := 4096 + i*40
		body[off+8] = byte(i + 1)
	}
	known, n, err := parseKnownFromPrevious(body)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2 chunks, got %d", n)
	}
	var d [32]byte
	d[0] = 1
	if !known.Has(d) {
		t.Fatal("expected digest 1")
	}
	d[0] = 2
	if !known.Has(d) {
		t.Fatal("expected digest 2")
	}
}
