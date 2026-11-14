package domain

import (
	"time"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type Kind string

const (
	// Fixed happens at a set time and the planner only works around it.
	Fixed Kind = "fixed"
	// Recurring is done a number of times a week, on any days; the user
	// places each session in the week.
	Recurring Kind = "recurring"
	// OneOff is done once, by a deadline, with no estimate.
	OneOff Kind = "one_off"
)

const (
	MinDuration        = 5 * time.Minute
	MaxSessionDuration = 12 * time.Hour
)

// Schedule says when a task happens. It is one of FixedSchedule,
// RecurringSchedule or OneOffSchedule.
type Schedule interface {
	Kind() Kind
	validate() error
}

// FixedSchedule repeats on Weekdays or happens once on Date, never both. An
// End before Start means it ends the next day, as in a night's sleep.
type FixedSchedule struct {
	Weekdays calendar.Weekdays
	Date     calendar.Date
	Start    calendar.TimeOfDay
	End      calendar.TimeOfDay
}

func (FixedSchedule) Kind() Kind { return Fixed }

func (s FixedSchedule) validate() error {
	if s.Weekdays.IsEmpty() == s.Date.IsZero() {
		return ErrFixedTaskDays
	}
	if !s.Start.Valid() || !s.End.Valid() {
		return calendar.ErrInvalidTime
	}
	if s.End == s.Start {
		return ErrInvalidTimeRange
	}
	return nil
}

// MaxTimesPerWeek allows one session a day.
const MaxTimesPerWeek = 7

// RecurringSchedule is done TimesPerWeek times a week, on any days, taking
// Duration each time. Weekdays are the days the user usually does it, and
// Period the part of the day the user prefers; both may be empty.
type RecurringSchedule struct {
	TimesPerWeek int
	Weekdays     calendar.Weekdays
	Duration     time.Duration
	Period       calendar.Period
}

func (RecurringSchedule) Kind() Kind { return Recurring }

func (s RecurringSchedule) validate() error {
	if s.TimesPerWeek < 1 || s.TimesPerWeek > MaxTimesPerWeek {
		return ErrInvalidTimesPerWeek
	}
	if s.Duration < MinDuration || s.Duration > MaxSessionDuration {
		return ErrInvalidSessionDuration
	}
	return validatePeriod(s.Period)
}

// OneOffSchedule is done by the end of Deadline, preferably in Period, which
// may be empty.
type OneOffSchedule struct {
	Deadline calendar.Date
	Period   calendar.Period
}

func (OneOffSchedule) Kind() Kind { return OneOff }

func (s OneOffSchedule) validate() error {
	if s.Deadline.IsZero() {
		return ErrDeadlineRequired
	}
	return validatePeriod(s.Period)
}

func validatePeriod(p calendar.Period) error {
	if p != "" && !p.Valid() {
		return ErrInvalidPeriod
	}
	return nil
}
