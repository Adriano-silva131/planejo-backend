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
		Schedule: taskdomain.RecurringSchedule{TimesPerWeek: 1, Weekdays: days, Duration: time.Hour},
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

func TestToPlannableTask(t *testing.T) {
	id := uuid.New()
	run := taskdomain.Task{ID: id, Details: taskdomain.Details{
		Title: "Corrida", Area: "Saúde", Priority: taskdomain.Medium,
		Schedule: taskdomain.RecurringSchedule{TimesPerWeek: 2, Duration: 30 * time.Minute, Period: calendar.Morning},
	}}
	report := taskdomain.Task{ID: id, Details: taskdomain.Details{
		Title: "Relatório", Area: "Trabalho", Priority: taskdomain.Low,
		Schedule: taskdomain.OneOffSchedule{Deadline: calendar.NewDate(2026, time.November, 5)},
	}}
	work := taskdomain.Task{ID: id, Details: taskdomain.Details{
		Title: "Trabalho", Area: "Trabalho",
		Schedule: taskdomain.FixedSchedule{Weekdays: calendar.Weekdays(1 << time.Monday), Start: 9 * 60, End: 18 * 60},
	}}

	recurring, ok := toPlannableTask(t.Context(), run)
	require.True(t, ok)
	oneOff, ok := toPlannableTask(t.Context(), report)
	require.True(t, ok)
	_, fixed := toPlannableTask(t.Context(), work)

	assert.Equal(t, routinedomain.PlannableTask{
		ID: id, Title: "Corrida", Area: "Saúde", Kind: routinedomain.RecurringKind, Duration: 30 * time.Minute,
		Period: calendar.Morning,
	}, recurring)
	assert.Equal(t, routinedomain.PlannableTask{
		ID: id, Title: "Relatório", Area: "Trabalho", Kind: routinedomain.OneOffKind,
		Deadline: calendar.NewDate(2026, time.November, 5),
	}, oneOff)
	assert.False(t, fixed, "a fixed task has occurrences, not sessions")
}

func TestToPlannableTaskUsesCreationDayInUserTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	task := taskdomain.Task{Details: taskdomain.Details{
		Schedule: taskdomain.OneOffSchedule{Deadline: calendar.NewDate(2026, time.November, 5)},
	}, CreatedAt: time.Date(2026, time.October, 26, 1, 0, 0, 0, time.UTC)}

	got, ok := toPlannableTask(calendar.WithLocation(t.Context(), loc), task)

	require.True(t, ok)
	assert.Equal(t, calendar.NewDate(2026, time.October, 25), got.CreatedOn)
}
