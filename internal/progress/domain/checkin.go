package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

const MaxMinutesPerDay = 24 * 60

// CheckIn records that a task was done on a day. A task has at most one
// check-in per day, on the day it really happened, which may differ from the
// days the task is set for.
type CheckIn struct {
	UserID uuid.UUID
	TaskID uuid.UUID
	Date   calendar.Date
	// Minutes worked that day and Completed apply only to one-off tasks.
	Minutes   int
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Entry is what the user reports for a day.
type Entry struct {
	Date      calendar.Date
	Minutes   int
	Completed bool
}

// NewCheckIn marks task as done. today is the latest day the user can be in:
// a day after it has not happened yet.
func NewCheckIn(userID uuid.UUID, task Task, e Entry, today calendar.Date) (CheckIn, error) {
	switch {
	case task.Kind == FixedTask:
		return CheckIn{}, ErrFixedTaskCheckIn
	case e.Date.IsZero():
		return CheckIn{}, calendar.ErrInvalidDate
	case e.Date.After(today):
		return CheckIn{}, ErrFutureCheckIn
	case task.Kind != OneOffTask && (e.Minutes != 0 || e.Completed):
		return CheckIn{}, ErrOneOffOnly
	case e.Minutes < 0 || e.Minutes > MaxMinutesPerDay:
		return CheckIn{}, ErrInvalidMinutes
	}
	return CheckIn{
		UserID:    userID,
		TaskID:    task.ID,
		Date:      e.Date,
		Minutes:   e.Minutes,
		Completed: e.Completed,
	}, nil
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
	// List returns the matching check-ins, oldest first.
	List(ctx context.Context, userID uuid.UUID, f Filter) ([]CheckIn, error)
	// SaveSchedule keeps the schedule a task had until s.Until. When the
	// schedule changes more than once in a week, the first one is kept.
	SaveSchedule(ctx context.Context, userID, taskID uuid.UUID, s PastSchedule) error
	// Schedules returns the past schedules of the user's tasks, oldest first.
	Schedules(ctx context.Context, userID uuid.UUID) (map[uuid.UUID][]PastSchedule, error)
}
