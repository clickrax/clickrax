package notify

import "testing"

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		mode   string
		status string
		want   bool
	}{
		{"off", "ok", false},
		{"off", "error", false},
		{"always", "ok", true},
		{"always", "error", true},
		{"failure", "ok", false},
		{"failure", "warning", true},
		{"failure", "error", true},
		{"failure", "cancelled", true},
	}
	for _, tc := range tests {
		if got := ShouldNotify(tc.mode, tc.status); got != tc.want {
			t.Fatalf("ShouldNotify(%q,%q)=%v want %v", tc.mode, tc.status, got, tc.want)
		}
	}
}

func TestEffectiveNotifyMode(t *testing.T) {
	if got := EffectiveNotifyMode("inherit", "always"); got != NotifyAlways {
		t.Fatalf("inherit+always: got %q", got)
	}
	if got := EffectiveNotifyMode("off", "always"); got != NotifyOff {
		t.Fatalf("off overrides always: got %q", got)
	}
	if got := EffectiveNotifyMode("failure", "off"); got != NotifyFailure {
		t.Fatalf("job failure overrides global off: got %q", got)
	}
}

func TestParseRecipients(t *testing.T) {
	got := parseRecipients("a@x.com, b@y.com , ")
	if len(got) != 2 || got[0] != "a@x.com" || got[1] != "b@y.com" {
		t.Fatalf("parseRecipients: %#v", got)
	}
}
