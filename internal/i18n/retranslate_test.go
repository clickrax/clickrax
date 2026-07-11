package i18n

import "testing"

func TestRetranslateStoredError(t *testing.T) {
	en := New("en")
	msg := "загрузка предыдущего индекса: connection refused"
	got := en.RetranslateStored(msg)
	want := "loading previous index: connection refused"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRetranslateJobName(t *testing.T) {
	en := New("en")
	got := en.RetranslateJobName("Быстрый 06.07.2026 22:03")
	if got != "Quick 06.07.2026 22:03" {
		t.Fatalf("got %q", got)
	}
}

func TestRetranslateFastIncSkipped(t *testing.T) {
	en := New("en")
	ruMsg := "Быстрый инкремент: пропущено 1 файлов; chunks новых 4878, переиспользовано 272"
	got := en.RetranslateStored(ruMsg)
	want := "Fast incremental: skipped 1 files; new chunks 4878, reused 272"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRetranslatePBSStageExact(t *testing.T) {
	en := New("en")
	got := en.RetranslateStored("Подготовка VSS...")
	want := "Preparing VSS..."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeBackupType(t *testing.T) {
	if NormalizeBackupType("инкрементальный") != "incremental" {
		t.Fatal("expected incremental")
	}
}
