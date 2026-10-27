package main

import (
	"context"

	"github.com/google/uuid"

	progressapp "github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	routineapp "github.com/Adriano-silva131/planejo-backend/internal/routine/app"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// routineListener tells the routine about a schedule change, with the task as
// the routine sees it.
type routineListener struct {
	routine *routineapp.Routine
}

func (l routineListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, _, after taskdomain.Task) error {
	if fixed, ok := toFixedTask(after); ok {
		return l.routine.ScheduleChanged(ctx, userID, after.ID, &fixed)
	}
	return l.routine.ScheduleChanged(ctx, userID, after.ID, nil)
}

// progressListener tells progress the weekdays a habit had and has.
type progressListener struct {
	progress *progressapp.Progress
}

func (l progressListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, before, after taskdomain.Task) error {
	return l.progress.ScheduleChanged(ctx, userID, after.ID, recurringWeekdays(before), recurringWeekdays(after))
}

// recurringWeekdays returns the weekdays of a recurring task; empty for others.
func recurringWeekdays(t taskdomain.Task) calendar.Weekdays {
	if s, ok := t.Schedule.(taskdomain.RecurringSchedule); ok {
		return s.Weekdays
	}
	return 0
}
