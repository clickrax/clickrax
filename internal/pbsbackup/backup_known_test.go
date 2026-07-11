package pbsbackup

import (
	"bytes"
	"testing"
)

func TestParseKnownFromPreviousMagicOnly(t *testing.T) {
	body := append(append([]byte(nil), didxMagic...), make([]byte, 48)...)
	known, n, err := parseKnownFromPrevious(body)
	if err != nil {
		t.Fatalf("short index should degrade gracefully: %v", err)
	}
	if n != 0 || known.Len() != 0 {
		t.Fatalf("expected empty known set, got %d chunks", n)
	}
}

func TestParseKnownFromPreviousTruncatedBody(t *testing.T) {
	body := make([]byte, 4096+48)
	copy(body, didxMagic)
	_, _, err := parseKnownFromPrevious(body)
	if err != nil {
		t.Fatalf("partial tail should be ignored without panic: %v", err)
	}
}

func TestParseKnownFromPreviousOneChunk(t *testing.T) {
	body := make([]byte, 4096+40)
	copy(body, didxMagic)
	copy(body[4096+8:4096+40], bytes.Repeat([]byte{0xab}, 32))
	known, n, err := parseKnownFromPrevious(body)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || known.Len() != 1 {
		t.Fatalf("expected 1 chunk, got %d", n)
	}
}
