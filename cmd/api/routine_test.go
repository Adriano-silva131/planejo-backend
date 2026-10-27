package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	routinedomain "github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

func TestToFixedTask(t *testing.T) {
	days, err := calendar.NewWeekdays(time.Sunday)
	require.NoError(t, err)
	id := uuid.New()
	sleep := taskdomain.Task{ID: id, Details: taskdomain.Details{
		Title: "Dormir", Area: "Descanso",
		Schedule: taskdomain.FixedSchedule{Weekdays: days, Start: 23 * 60, End: 7 * 60},
	}, CreatedAt: time.Date(2026, time.October, 27, 1, 30, 0, 0, time.UTC)}
	gym := taskdomain.Task{ID: uuid.New(), Details: taskdomain.Details{
		Title: "Academia", Area: "Saúde", Priority: taskdomain.High,
		Schedule: taskdomain.RecurringSchedule{Weekdays: days, Duration: time.Hour},
	}}

	fixed, ok := toFixedTask(sleep)
	_, recurring := toFixedTask(gym)

	require.True(t, ok)
	assert.Equal(t, routinedomain.FixedTask{
		ID: id, Title: "Dormir", Area: "Descanso", Weekdays: days, Start: 23 * 60, End: 7 * 60,
		CreatedOn: calendar.NewDate(2026, time.October, 27),
	}, fixed)
	assert.False(t, recurring, "only fixed tasks are part of the routine")
}
