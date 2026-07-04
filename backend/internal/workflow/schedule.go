package workflow

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// ParseCronSchedule validates a standard 5-field cron expression (minute
// hour day-of-month month day-of-week - no seconds field) and returns a
// cron.Schedule that can compute the next run time from any point. Kept as
// a thin wrapper so the rest of the package (and its callers) depend on one
// place for "what does a valid schedule look like", not the cron library
// directly.
func ParseCronSchedule(expr string) (cron.Schedule, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return sched, nil
}

// NextRun returns the next time expr fires strictly after from.
func NextRun(expr string, from time.Time) (time.Time, error) {
	sched, err := ParseCronSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}
