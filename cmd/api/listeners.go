package main

import (
	"context"

	"github.com/google/uuid"

	progressapp "github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	routineapp "github.com/Adriano-silva131/planejo-backend/internal/routine/app"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// routineListener drops the routine adjustments that no longer apply.
type routineListener struct {
	routine *routineapp.Routine
}

func (l routineListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, _, after taskdomain.Task) error {
	return l.routine.ScheduleChanged(ctx, userID, after.ID)
}

// progressListener keeps the weekdays a habit had, so its past weeks are
// judged by them.
type progressListener struct {
	progress *progressapp.Progress
}

func (l progressListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, before, after taskdomain.Task) error {
	weekdays, ok := pastHabitWeekdays(before, after)
	if !ok {
		return nil
	}
	return l.progress.ScheduleChanged(ctx, userID, after.ID, weekdays)
}

// pastHabitWeekdays returns the weekdays to keep for a schedule change that
// matters to habits: one into, out of or within a recurring schedule that
// changes its weekdays. Empty weekdays mean the task was not recurring.
func pastHabitWeekdays(before, after taskdomain.Task) (calendar.Weekdays, bool) {
	b, wasRecurring := before.Schedule.(taskdomain.RecurringSchedule)
	a, isRecurring := after.Schedule.(taskdomain.RecurringSchedule)
	switch {
	case !wasRecurring && !isRecurring:
		return 0, false
	case wasRecurring && isRecurring && b.Weekdays == a.Weekdays:
		return 0, false
	case !wasRecurring:
		return 0, true
	}
	return b.Weekdays, true
}
