package notify

import (
	"strings"
	"testing"
)

func TestSanitizeHeaderStripsCRLF(t *testing.T) {
	got := sanitizeHeader("job\r\nBcc: evil@x.com")
	if strings.Contains(got, "\r") || strings.Contains(got, "\n") {
		t.Fatalf("CRLF not stripped: %q", got)
	}
	if !strings.Contains(got, "Bcc:") {
		t.Fatalf("expected flattened value, got %q", got)
	}
}

func TestEncodeSubjectASCII(t *testing.T) {
	got := encodeSubject("Backup OK")
	if got != "Backup OK" {
		t.Fatalf("ASCII subject should pass through, got %q", got)
	}
}

func TestEncodeSubjectUTF8(t *testing.T) {
	got := encodeSubject("Успешно — тест")
	if got == "Успешно — тест" {
		t.Fatal("UTF-8 subject should be encoded")
	}
	if !strings.HasPrefix(got, "=?utf-8?") {
		t.Fatalf("expected RFC2047 encoding, got %q", got)
	}
}

func TestEnvelopeAddrDisplayName(t *testing.T) {
	got, err := envelopeAddr("ClickRAX <notify@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	if got != "notify@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildMessageNoHeaderInjection(t *testing.T) {
	msg, err := buildMessage(
		"from@example.com",
		[]string{"to@example.com"},
		"subj\r\nX-Evil: 1",
		"body",
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg, "\r\nX-Evil:") {
		t.Fatalf("injected header in message: %q", msg)
	}
}
