package pbsbackup

import "testing"

func TestResolveOriginalDest_singleSource(t *testing.T) {
	sources := []string{`D:\backup-source`}
	got, err := ResolveOriginalDest(`example.com\api.php`, sources)
	if err != nil {
		t.Fatal(err)
	}
	want := `D:\backup-source\example.com\api.php`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveOriginalDest_absoluteUnderSource(t *testing.T) {
	sources := []string{`C:\backup`}
	got, err := ResolveOriginalDest(`C:\backup\absolute.txt`, sources)
	if err != nil {
		t.Fatal(err)
	}
	if got != `C:\backup\absolute.txt` {
		t.Fatalf("got %q", got)
	}
}

func TestResolveOriginalDest_absoluteOutsideSource(t *testing.T) {
	sources := []string{`D:\backup`}
	if _, err := ResolveOriginalDest(`C:\already\absolute.txt`, sources); err == nil {
		t.Fatal("expected error for path outside sources")
	}
}

func TestResolveOriginalDest_volumeSource(t *testing.T) {
	sources := []string{`D:\`}
	got, err := ResolveOriginalDest(`folder\file.txt`, sources)
	if err != nil {
		t.Fatal(err)
	}
	want := `D:\folder\file.txt`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
