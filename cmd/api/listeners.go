package main

import (
	"context"

	"github.com/google/uuid"

	progressapp "github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	progressdomain "github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	routineapp "github.com/Adriano-silva131/planejo-backend/internal/routine/app"
	routinedomain "github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// routineListener tells the routine about a schedule change, with the task as
// the routine sees it when it is fixed.
type routineListener struct {
	routine *routineapp.Routine
}

func (l routineListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, _, after taskdomain.Task) error {
	var fixed *routinedomain.FixedTask
	if f, ok := toFixedTask(after); ok {
		fixed = &f
	}
	return l.routine.ScheduleChanged(ctx, userID, after.ID, fixed)
}

// routineChecker refuses a fixed task that makes too many things happen at
// once in the routine.
type routineChecker struct {
	routine *routineapp.Routine
}

func (c routineChecker) CheckSchedule(ctx context.Context, userID uuid.UUID, task taskdomain.Task) error {
	if fixed, ok := toFixedTask(task); ok {
		return c.routine.CheckFixed(ctx, userID, fixed)
	}
	return nil
}

// progressListener tells progress how a habit repeated and repeats, and when
// a task became one-off.
type progressListener struct {
	progress *progressapp.Progress
}

func (l progressListener) ScheduleChanged(ctx context.Context, userID uuid.UUID, before, after taskdomain.Task) error {
	if err := l.progress.ScheduleChanged(ctx, userID, after.ID, recurrence(before), recurrence(after)); err != nil {
		return err
	}
	if isOneOff(after) && !isOneOff(before) {
		return l.progress.BecameOneOff(ctx, userID, after.ID)
	}
	return nil
}

func isOneOff(t taskdomain.Task) bool {
	return t.Schedule.Kind() == taskdomain.OneOff
}

// recurrence returns how a recurring task repeats; zero for others.
func recurrence(t taskdomain.Task) progressdomain.Recurrence {
	if s, ok := t.Schedule.(taskdomain.RecurringSchedule); ok {
		return progressdomain.Recurrence{TimesPerWeek: s.TimesPerWeek, Weekdays: s.Weekdays}
	}
	return progressdomain.Recurrence{}
}
