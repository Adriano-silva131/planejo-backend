package domain

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Habit is how a recurring task is set for one week: Target times, usually on
// Weekdays. The task exists from Since on; days before it ask for nothing.
type Habit struct {
	TaskID   uuid.UUID
	Target   int
	Weekdays calendar.Weekdays
	Since    calendar.Date
}

// CheckIn is a day a task was done, started at Start when the user said.
type CheckIn struct {
	TaskID uuid.UUID
	Date   calendar.Date
	Start  calendar.OptionalTime
}

// WeekProgress is what the progress module knows of a week: how the
// recurring tasks are set for it, the check-ins of its days and the one-off
// tasks completed on any day.
type WeekProgress struct {
	Habits    []Habit
	CheckIns  []CheckIn
	Completed map[uuid.UUID]bool
}

// Progress reads the progress module, in the user's time zone stored in ctx.
type Progress interface {
	// Habits returns how the recurring tasks are set for the week starting on
	// monday; a task that was not recurring then is left out.
	Habits(ctx context.Context, userID uuid.UUID, monday calendar.Date) ([]Habit, error)
	// CheckIns returns the check-ins from from to to.
	CheckIns(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]CheckIn, error)
	// Completed returns which of the one-off tasks ids were completed.
	Completed(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}
