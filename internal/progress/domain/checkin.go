package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// CheckIn records that a task was done on a day. A task has at most one
// check-in per day, on the day it really happened, which may differ from the
// days the task is set for. A one-off task has at most one check-in: the day
// it was completed.
type CheckIn struct {
	UserID uuid.UUID
	TaskID uuid.UUID
	Date   calendar.Date
	// Start is when the task was really started that day, if the user said.
	Start     calendar.OptionalTime
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Entry is what the user reports for a day.
type Entry struct {
	Date  calendar.Date
	Start calendar.OptionalTime
}

// NewCheckIn marks task as done. today is the latest day the user can be in:
// a day after it has not happened yet. existing are the task's check-ins so
// far, which keep a one-off task from being completed twice.
func NewCheckIn(userID uuid.UUID, task Task, e Entry, today calendar.Date, existing []CheckIn) (CheckIn, error) {
	switch {
	case task.Kind == FixedTask:
		return CheckIn{}, ErrFixedTaskCheckIn
	case e.Date.IsZero():
		return CheckIn{}, calendar.ErrInvalidDate
	case e.Date.After(today):
		return CheckIn{}, ErrFutureCheckIn
	case !e.Start.Valid():
		return CheckIn{}, calendar.ErrInvalidTime
	case task.Kind == OneOffTask && completedOtherDay(existing, e.Date):
		return CheckIn{}, ErrAlreadyCompleted
	}
	return CheckIn{UserID: userID, TaskID: task.ID, Date: e.Date, Start: e.Start}, nil
}

func completedOtherDay(checkIns []CheckIn, date calendar.Date) bool {
	for _, c := range checkIns {
		if c.Date != date {
			return true
		}
	}
	return false
}

// Filter narrows a listing. Zero fields do not filter; dates are inclusive.
type Filter struct {
	TaskID uuid.UUID
	// TaskIDs keeps only the check-ins of these tasks.
	TaskIDs []uuid.UUID
	From    calendar.Date
	To      calendar.Date
}

// Repository stores check-ins, always scoped to one user.
type Repository interface {
	// Save creates the check-in, or replaces the one of the same task and day.
	// It fails with ErrTaskNotFound when the task is not the user's, or was
	// deleted meanwhile.
	Save(ctx context.Context, c CheckIn) (CheckIn, error)
	Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error
	DeleteForTask(ctx context.Context, userID, taskID uuid.UUID) error
	// LockTask makes other check-ins of the task wait until the transaction
	// in ctx ends.
	LockTask(ctx context.Context, userID, taskID uuid.UUID) error
	// List returns the matching check-ins, oldest first.
	List(ctx context.Context, userID uuid.UUID, f Filter) ([]CheckIn, error)
	// SaveSchedule keeps the schedule a task had until s.Until. When the
	// schedule changes more than once in a week, the first one is kept.
	SaveSchedule(ctx context.Context, userID, taskID uuid.UUID, s PastSchedule) error
	// Schedules returns the past schedules of the user's tasks, oldest first.
	Schedules(ctx context.Context, userID uuid.UUID) (map[uuid.UUID][]PastSchedule, error)
}
