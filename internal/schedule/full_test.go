package schedule

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestShouldForceFull(t *testing.T) {
	sun := time.Date(2026, 7, 5, 2, 0, 0, 0, time.Local) // Sunday
	mon := time.Date(2026, 7, 6, 2, 0, 0, 0, time.Local)

	sch := models.Schedule{
		Enabled:           true,
		Type:              "daily",
		FullBackupMode:    "weekly",
		FullBackupWeekday: 7,
	}

	if !ShouldForceFull(sch, sun) {
		t.Fatal("Sunday should be full")
	}
	if ShouldForceFull(sch, mon) {
		t.Fatal("Monday should be incremental")
	}

	sch.FullBackupMode = "never"
	if ShouldForceFull(sch, sun) {
		t.Fatal("never mode should not force full")
	}

	sch.FullBackupMode = "weekly"
	sch.Type = "weekly"
	sch.Weekdays = []int{1, 2, 3, 4, 5}
	if ShouldForceFull(sch, sun) {
		t.Fatal("Sunday not in weekly list")
	}
}

func TestShouldForceFullBiweekly(t *testing.T) {
	anchor := time.Date(2026, 7, 5, 0, 0, 0, 0, time.Local) // Sunday
	sch := models.Schedule{
		Type:              "daily",
		FullBackupMode:    "biweekly",
		FullBackupWeekday: 7,
		FullBackupAnchor:  "2026-07-05",
	}
	if !ShouldForceFull(sch, anchor) {
		t.Fatal("anchor Sunday should be full")
	}
	next := anchor.AddDate(0, 0, 7)
	if ShouldForceFull(sch, next) {
		t.Fatal("one week later should be incremental")
	}
	two := anchor.AddDate(0, 0, 14)
	if !ShouldForceFull(sch, two) {
		t.Fatal("two weeks later should be full")
	}
}

func TestShouldForceFullMonthly(t *testing.T) {
	sch := models.Schedule{
		Type:              "daily",
		FullBackupMode:    "monthly",
		FullBackupWeekday: 7,
	}
	firstSun := time.Date(2026, 7, 5, 2, 0, 0, 0, time.Local)
	secondSun := time.Date(2026, 7, 12, 2, 0, 0, 0, time.Local)
	if !ShouldForceFull(sch, firstSun) {
		t.Fatal("first Sunday of month should be full")
	}
	if ShouldForceFull(sch, secondSun) {
		t.Fatal("second Sunday of month should be incremental")
	}
}

func TestReconcileSchedulePreservesDisabled(t *testing.T) {
	s := models.Schedule{
		Enabled: false,
		Type:    "daily",
		Times:   []string{"09:53"},
	}
	ReconcileSchedule(&s)
	if s.Enabled {
		t.Fatal("expected schedule to stay disabled when user turned it off")
	}
	if s.Type != "daily" {
		t.Fatalf("type=%q", s.Type)
	}
}

func TestReconcileScheduleEmptyTypeWithTimes(t *testing.T) {
	s := models.Schedule{
		Enabled: false,
		Times:   []string{"08:00"},
	}
	ReconcileSchedule(&s)
	if s.Type != "daily" {
		t.Fatalf("type=%q want daily", s.Type)
	}
	if s.Enabled {
		t.Fatal("expected disabled schedule to remain off")
	}
}
