package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Adjustment changes one occurrence of a fixed task, the one that starts on
// Date, without changing the task: the occurrence is skipped, or happens from
// Start to End instead.
type Adjustment struct {
	UserID    uuid.UUID
	TaskID    uuid.UUID
	Date      calendar.Date
	Skipped   bool
	Start     calendar.TimeOfDay
	End       calendar.TimeOfDay
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Change is what the user asks for one occurrence: skip it, or move it to
// Start–End. An End before Start ends the next day.
type Change struct {
	Skip  bool
	Start calendar.TimeOfDay
	End   calendar.TimeOfDay
}

func NewAdjustment(userID uuid.UUID, task FixedTask, date calendar.Date, c Change) (Adjustment, error) {
	if !task.OccursOn(date) {
		return Adjustment{}, ErrNotAnOccurrence
	}
	a := Adjustment{UserID: userID, TaskID: task.ID, Date: date, Skipped: true}
	if c.Skip {
		return a, nil
	}
	if !c.Start.Valid() || !c.End.Valid() {
		return Adjustment{}, calendar.ErrInvalidTime
	}
	if c.Start == c.End {
		return Adjustment{}, ErrInvalidTimeRange
	}
	a.Skipped, a.Start, a.End = false, c.Start, c.End
	return a, nil
}

// Repository stores adjustments, always scoped to one user.
type Repository interface {
	// Save creates the adjustment, or replaces the one of the same occurrence.
	// It fails with ErrTaskNotFound when the task is not the user's, or was
	// deleted meanwhile.
	Save(ctx context.Context, a Adjustment) (Adjustment, error)
	Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error
	// List returns the adjustments of occurrences starting from from to to.
	List(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]Adjustment, error)
	// ForTask returns every adjustment of the task.
	ForTask(ctx context.Context, userID, taskID uuid.UUID) ([]Adjustment, error)
}
