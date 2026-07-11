package backuprunner

import (
	"strings"
	"testing"

	"pbs-win-backup/internal/i18n"
)

func TestFriendlyError_Auth(t *testing.T) {
	b := i18n.New("en")
	msg := FriendlyError(b, errString("authentication error: bad token"))
	if !strings.Contains(msg, "auth") && msg == "authentication error: bad token" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestShortenErr_TruncatesLongMessage(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := ShortenErr(errString(long))
	if len(got) > 130 {
		t.Fatalf("expected truncated message, got len %d", len(got))
	}
}

type errString string

func (e errString) Error() string { return string(e) }
