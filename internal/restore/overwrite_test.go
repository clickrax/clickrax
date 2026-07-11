package restore

import "testing"

func TestResolveFileDest(t *testing.T) {
	got, err := resolveFileDest(`TUNNEL\tun_back.sh`, `C:\restore\dest`)
	if err != nil {
		t.Fatal(err)
	}
	want := `C:\restore\dest\TUNNEL\tun_back.sh`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveFileDestEmptyRoot(t *testing.T) {
	if _, err := resolveFileDest(`D:\data\file.txt`, ""); err == nil {
		t.Fatal("expected error for empty dest root")
	}
}
