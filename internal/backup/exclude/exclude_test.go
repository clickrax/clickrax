package exclude

import "testing"

func TestMatchPathBasenameGlob(t *testing.T) {
	e := New([]string{"*.tmp"})
	if !e.MatchPath(`D:\a\b.tmp`, "b.tmp", false) {
		t.Fatal("expected *.tmp match")
	}
	if e.MatchPath(`D:\a\b.dat`, "b.dat", false) {
		t.Fatal("unexpected match")
	}
}

func TestMatchPathAnchoredFolder(t *testing.T) {
	e := NewForRoot(`D:\`, []string{`D:\Games`, `D:\Temp`})
	if !e.MatchPath(`D:\Games\save.bin`, "save.bin", false) {
		t.Fatal("expected Games subtree match")
	}
	if e.MatchPath(`D:\Work\Games\save.bin`, "save.bin", false) {
		t.Fatal("nested Games should not match anchored rule")
	}
	if !e.MatchPath(`D:\Temp\log.txt`, "log.txt", false) {
		t.Fatal("expected Temp match")
	}
}

func TestMatchPathSystemName(t *testing.T) {
	e := New(nil)
	if !e.MatchPath(`D:\System Volume Information\x`, "System Volume Information", true) {
		t.Fatal("expected system folder skip")
	}
}

func TestMergeDedupes(t *testing.T) {
	got := Merge([]string{"*.tmp", "Games"}, []string{"games", "*.log"})
	if len(got) != 3 {
		t.Fatalf("merge len %d", len(got))
	}
}
