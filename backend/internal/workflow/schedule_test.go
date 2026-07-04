package workflow

import (
	"testing"
	"time"
)

func TestParseCronScheduleRejectsGarbage(t *testing.T) {
	if _, err := ParseCronSchedule("not a cron expression"); err == nil {
		t.Fatal("expected an error for a malformed cron expression")
	}
}

func TestParseCronScheduleAcceptsStandardFiveField(t *testing.T) {
	if _, err := ParseCronSchedule("*/5 * * * *"); err != nil {
		t.Fatalf("expected a valid 5-field expression to parse, got %v", err)
	}
}

func TestNextRunIsStrictlyAfterFrom(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := NextRun("0 * * * *", from) // top of every hour
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected next run %v, got %v", want, next)
	}
}

func TestNextRunPropagatesParseError(t *testing.T) {
	_, err := NextRun("garbage", time.Now())
	if err == nil {
		t.Fatal("expected an error for a malformed cron expression")
	}
}
