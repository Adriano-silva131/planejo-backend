package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// FixedTask is what the routine needs to know about a fixed task of the
// task module: it happens on Weekdays, or once on Date, from Start to End. An
// End before Start ends the next day.
type FixedTask struct {
	ID       uuid.UUID
	Title    string
	Area     string
	Weekdays calendar.Weekdays
	Date     calendar.Date
	Start    calendar.TimeOfDay
	End      calendar.TimeOfDay
	// CreatedOn is the day the task was created, in UTC.
	CreatedOn calendar.Date
}

// OccursOn reports whether an occurrence of the task starts on d. A weekly
// task occurs from the week it was created on; a dated one, on its date.
func (t FixedTask) OccursOn(d calendar.Date) bool {
	if !t.Date.IsZero() {
		return t.Date.Equal(d)
	}
	return t.Weekdays.Has(d.Weekday()) && !d.Before(t.CreatedOn.StartOfWeek())
}

// PlannableTask is what the routine needs to know about a recurring or
// one-off task: the user places its sessions in the week. Duration is how long
// a recurring session lasts; a one-off task has none, but has a Deadline.
// Period is the part of the day the user prefers, which may be empty.
type PlannableTask struct {
	ID       uuid.UUID
	Title    string
	Area     string
	Kind     Kind
	Duration time.Duration
	Period   calendar.Period
	Deadline calendar.Date
	// CreatedOn is the day the task was created in the user's time zone.
	CreatedOn calendar.Date
}

// Tasks reads the user's tasks. List returns all of them, the fixed apart
// from the recurring and one-off ones. GetFixed
// and GetPlannable fail with a not-found error when the task does not exist
// or belongs to someone else, and with ErrNoOccurrences or
// ErrFixedTaskSession when it is of the other kind.
type Tasks interface {
	List(ctx context.Context, userID uuid.UUID) ([]FixedTask, []PlannableTask, error)
	GetFixed(ctx context.Context, userID, id uuid.UUID) (FixedTask, error)
	GetPlannable(ctx context.Context, userID, id uuid.UUID) (PlannableTask, error)
}
