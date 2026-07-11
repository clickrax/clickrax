package backup

import "testing"

func TestIsVolumeRoot(t *testing.T) {
	cases := map[string]bool{
		`C:\`:        true,
		`c:\`:        true,
		`D:`:         true,
		`E:/`:        true,
		`C:\Users`:   false,
		`/mnt/data`:  false,
		``:           false,
	}
	for path, want := range cases {
		if got := IsVolumeRoot(path); got != want {
			t.Fatalf("IsVolumeRoot(%q)=%v want %v", path, got, want)
		}
	}
}
