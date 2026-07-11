package service

import "testing"

func TestNormalizeServicePath_ignoresArgs(t *testing.T) {
	a := normalizeServicePath(`"C:\ProgramData\ClickRAX\bin\clickrax.exe" --service`)
	b := normalizeServicePath(`C:\ProgramData\ClickRAX\bin\clickrax.exe`)
	if a != b {
		t.Fatalf("got %q want %q", a, b)
	}
}

func TestServiceCommandLine_quotesPath(t *testing.T) {
	got := serviceCommandLine(`C:\ProgramData\ClickRAX\bin\clickrax.exe`)
	want := `"C:\ProgramData\ClickRAX\bin\clickrax.exe" --service`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
