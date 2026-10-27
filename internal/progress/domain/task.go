package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type TaskKind string

const (
	FixedTask     TaskKind = "fixed"
	RecurringTask TaskKind = "recurring"
	OneOffTask    TaskKind = "one_off"
)

// Task is what progress needs to know about a task of the task module.
type Task struct {
	ID    uuid.UUID
	Title string
	Area  string
	Kind  TaskKind
	// Weekdays and TimesPerWeek, their count, describe a recurring task.
	Weekdays     calendar.Weekdays
	TimesPerWeek int
	// TotalMinutes and Deadline describe a one-off task.
	TotalMinutes int
	Deadline     calendar.Date
	// CreatedAt is when the task was created. CreatedOn is that day in UTC,
	// or in the user's time zone when the caller knows it.
	CreatedAt time.Time
	CreatedOn calendar.Date
	// PastSchedules are the weekdays a recurring task had before its schedule
	// changed, oldest first.
	PastSchedules []PastSchedule
}

// PastSchedule says the task was set for Weekdays before its schedule
// changed on ChangedOn, a day in the user's time zone. Empty Weekdays means it
// was not a recurring task then.
type PastSchedule struct {
	ChangedOn calendar.Date
	Weekdays  calendar.Weekdays
}

// Until is the Monday of the week the change took effect: the weeks before it
// followed this schedule.
func (s PastSchedule) Until() calendar.Date {
	return s.ChangedOn.StartOfWeek()
}

// weekdaysOn returns the weekdays the task was set for in the week starting
// on start.
func (t Task) weekdaysOn(start calendar.Date) calendar.Weekdays {
	for _, s := range t.PastSchedules {
		if start.Before(s.Until()) {
			return s.Weekdays
		}
	}
	return t.Weekdays
}

// changeIn returns the schedule change that took effect in the week starting
// on start, if any.
func (t Task) changeIn(start calendar.Date) (PastSchedule, bool) {
	for _, s := range t.PastSchedules {
		if s.Until() == start {
			return s, true
		}
	}
	return PastSchedule{}, false
}

// Tasks reads the user's tasks. Get fails with a not-found error when the
// task does not exist or belongs to someone else.
type Tasks interface {
	Get(ctx context.Context, userID, id uuid.UUID) (Task, error)
	List(ctx context.Context, userID uuid.UUID) ([]Task, error)
}
