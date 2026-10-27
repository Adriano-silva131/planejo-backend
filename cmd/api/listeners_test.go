package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

func TestPastHabitWeekdays(t *testing.T) {
	mon := calendar.Weekdays(1 << time.Monday)
	monWed := mon | 1<<time.Wednesday
	recurring := func(days calendar.Weekdays, d time.Duration) taskdomain.Task {
		return taskdomain.Task{Details: taskdomain.Details{Schedule: taskdomain.RecurringSchedule{Weekdays: days, Duration: d}}}
	}
	fixed := taskdomain.Task{Details: taskdomain.Details{Schedule: taskdomain.FixedSchedule{Weekdays: mon, Start: 540, End: 600}}}

	cases := map[string]struct {
		before, after taskdomain.Task
		want          calendar.Weekdays
		keep          bool
	}{
		"new weekdays":        {recurring(mon, time.Hour), recurring(monWed, time.Hour), mon, true},
		"only duration":       {recurring(mon, time.Hour), recurring(mon, 2*time.Hour), 0, false},
		"became recurring":    {fixed, recurring(mon, time.Hour), 0, true},
		"no longer recurring": {recurring(monWed, time.Hour), fixed, monWed, true},
		"never recurring":     {fixed, fixed, 0, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, keep := pastHabitWeekdays(tc.before, tc.after)

			assert.Equal(t, tc.keep, keep)
			assert.Equal(t, tc.want, got)
		})
	}
}
