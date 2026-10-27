package domain

import (
	"context"

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
	// CreatedOn is the day the task was created, in UTC.
	CreatedOn calendar.Date
}

// Tasks reads the user's tasks. Get fails with a not-found error when the
// task does not exist or belongs to someone else.
type Tasks interface {
	Get(ctx context.Context, userID, id uuid.UUID) (Task, error)
	List(ctx context.Context, userID uuid.UUID) ([]Task, error)
}
