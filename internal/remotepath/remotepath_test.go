package remotepath

import "testing"

func TestSafeComponent_rejectsTraversal(t *testing.T) {
	bad := []string{"../secret.zip", `..\..\x`, "foo/bar", `a\b`, "a:b"}
	for _, name := range bad {
		if _, err := SafeComponent(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
	if got, err := SafeComponent("archive.zip"); err != nil || got != "archive.zip" {
		t.Fatalf("archive.zip: got %q err %v", got, err)
	}
}

func TestRemoteDir(t *testing.T) {
	dir, err := RemoteDir("backups/prod", "host-01")
	if err != nil || dir != "backups/prod/host-01" {
		t.Fatalf("got %q err %v", dir, err)
	}
	if _, err := RemoteDir("ok", "../bad"); err == nil {
		t.Fatal("expected backup id traversal error")
	}
}
