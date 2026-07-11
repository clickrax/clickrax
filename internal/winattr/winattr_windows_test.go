//go:build windows

package winattr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureApplyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("acl-test"), 0o644); err != nil {
		t.Fatal(err)
	}

	entry, err := Capture(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("acl-test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if entry.SDDL == "" && entry.Attributes == 0 {
		t.Skip("no security metadata captured (insufficient privileges?)")
	}
	if err := Apply(dst, entry); err != nil {
		t.Fatalf("apply: %v", err)
	}
}
