package domain

import (
	"context"

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

// Tasks reads the user's fixed tasks. Fixed returns those that may occur
// from from to to. Get fails with a not-found error when the task does not
// exist or belongs to someone else, and with ErrNotFixedTask when it is not
// fixed.
type Tasks interface {
	Fixed(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]FixedTask, error)
	Get(ctx context.Context, userID, id uuid.UUID) (FixedTask, error)
}
