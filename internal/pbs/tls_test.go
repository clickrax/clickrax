package pbs

import "testing"

func TestTLSConfigEmptyFingerprintUsesSystemTrust(t *testing.T) {
	cfg := tlsConfig("")
	if cfg.InsecureSkipVerify {
		t.Fatal("empty fingerprint must use system trust store, not InsecureSkipVerify")
	}
}

func TestTLSConfigWithFingerprintPinsCert(t *testing.T) {
	cfg := tlsConfig("ab:cd")
	if !cfg.InsecureSkipVerify {
		t.Fatal("pinned fingerprint requires InsecureSkipVerify with custom verify")
	}
	if cfg.VerifyConnection == nil {
		t.Fatal("expected VerifyConnection callback")
	}
}
