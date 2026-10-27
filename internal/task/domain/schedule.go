package domain

import (
	"time"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type Kind string

const (
	// Fixed happens at a set time and the planner only works around it.
	Fixed Kind = "fixed"
	// Recurring repeats on chosen weekdays; the planner picks the time.
	Recurring Kind = "recurring"
	// OneOff happens once before a deadline; the planner may split it.
	OneOff Kind = "one_off"
)

// Period is the part of the day the user prefers. Empty means no preference.
type Period string

const (
	Morning   Period = "morning"
	Afternoon Period = "afternoon"
	Evening   Period = "evening"
)

const (
	MinDuration        = 5 * time.Minute
	MaxSessionDuration = 12 * time.Hour
	MaxTotalDuration   = 100 * time.Hour
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

// RecurringSchedule takes Duration on each of the Weekdays.
type RecurringSchedule struct {
	Weekdays calendar.Weekdays
	Duration time.Duration
	Period   Period
}

func (RecurringSchedule) Kind() Kind { return Recurring }

func (s RecurringSchedule) validate() error {
	if s.Weekdays.IsEmpty() {
		return ErrWeekdaysRequired
	}
	if s.Duration < MinDuration || s.Duration > MaxSessionDuration {
		return ErrInvalidSessionDuration
	}
	return validatePeriod(s.Period)
}

// OneOffSchedule takes Duration in total, done by the end of Deadline.
type OneOffSchedule struct {
	Duration time.Duration
	Deadline calendar.Date
	Period   Period
}

func (OneOffSchedule) Kind() Kind { return OneOff }

func (s OneOffSchedule) validate() error {
	if s.Deadline.IsZero() {
		return ErrDeadlineRequired
	}
	if s.Duration < MinDuration || s.Duration > MaxTotalDuration {
		return ErrInvalidTotalDuration
	}
	return validatePeriod(s.Period)
}

func validatePeriod(p Period) error {
	switch p {
	case "", Morning, Afternoon, Evening:
		return nil
	}
	return ErrInvalidPeriod
}
