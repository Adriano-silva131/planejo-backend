package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	progressdomain "github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

func TestToProgressTask(t *testing.T) {
	days, err := calendar.NewWeekdays(time.Monday, time.Wednesday, time.Friday)
	require.NoError(t, err)
	deadline := calendar.NewDate(2026, time.December, 10)
	id := uuid.New()
	// 01:30 UTC on the 27th is still the 26th in São Paulo; CreatedOn uses UTC.
	createdAt := time.Date(2026, time.October, 27, 1, 30, 0, 0, time.UTC).In(time.FixedZone("BRT", -3*60*60))
	createdOn := calendar.NewDate(2026, time.October, 27)

	cases := map[string]struct {
		schedule taskdomain.Schedule
		want     progressdomain.Task
	}{
		"fixed": {
			taskdomain.FixedSchedule{Weekdays: days, Start: 9 * 60, End: 18 * 60},
			progressdomain.Task{ID: id, Title: "T", Kind: progressdomain.FixedTask, CreatedOn: createdOn},
		},
		"recurring": {
			taskdomain.RecurringSchedule{Weekdays: days, Duration: time.Hour},
			progressdomain.Task{
				ID: id, Title: "T", Kind: progressdomain.RecurringTask, TimesPerWeek: 3, CreatedOn: createdOn,
			},
		},
		"one-off": {
			taskdomain.OneOffSchedule{Duration: 20 * time.Hour, Deadline: deadline},
			progressdomain.Task{
				ID: id, Title: "T", Kind: progressdomain.OneOffTask, TotalMinutes: 1200, Deadline: deadline,
				CreatedOn: createdOn,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			task := taskdomain.Task{
				ID: id, Details: taskdomain.Details{Title: "T", Schedule: tc.schedule}, CreatedAt: createdAt,
			}

			assert.Equal(t, tc.want, toProgressTask(task))
		})
	}
}
