package notify

import "testing"

func TestValidateWebhookURLHTTPSOnly(t *testing.T) {
	if err := ValidateWebhookURL("http://example.com/hook"); err == nil {
		t.Fatal("http webhook should be rejected")
	}
}

func TestValidateWebhookURLBlocksLoopbackIP(t *testing.T) {
	if err := ValidateWebhookURL("https://127.0.0.1/hook"); err == nil {
		t.Fatal("loopback IP should be blocked")
	}
}

func TestValidateWebhookURLBlocksLocalhost(t *testing.T) {
	if err := ValidateWebhookURL("https://localhost/hook"); err == nil {
		t.Fatal("localhost should be blocked")
	}
}

func TestValidateWebhookURLAllowsPublicHost(t *testing.T) {
	// example.com resolves to public IPs in most environments
	if err := ValidateWebhookURL("https://example.com/hook"); err != nil {
		t.Fatalf("public host should be allowed: %v", err)
	}
}
