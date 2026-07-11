package pathsafe

import "testing"

func TestSafeRelativePath_rejectsTraversal(t *testing.T) {
	if _, err := SafeRelativePath(`..\windows\system32\cmd.exe`); err == nil {
		t.Fatal("expected error for ..")
	}
}

func TestSafeRelativePath_rejectsAbsolute(t *testing.T) {
	if _, err := SafeRelativePath(`C:\secret.txt`); err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestJoinUnderRoot_ok(t *testing.T) {
	got, err := JoinUnderRoot(`C:\restore`, `folder\file.txt`)
	if err != nil {
		t.Fatal(err)
	}
	want := `C:\restore\folder\file.txt`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJoinUnderRoot_escape(t *testing.T) {
	if _, err := JoinUnderRoot(`C:\restore`, `..\outside.txt`); err == nil {
		t.Fatal("expected escape error")
	}
}
