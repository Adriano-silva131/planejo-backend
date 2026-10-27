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
	Kind  TaskKind
	// TimesPerWeek is how many weekdays a recurring task is set for.
	TimesPerWeek int
	// TotalMinutes and Deadline describe a one-off task.
	TotalMinutes int
	Deadline     calendar.Date
}

// Tasks reads the user's tasks. Get fails with a not-found error when the
// task does not exist or belongs to someone else.
type Tasks interface {
	Get(ctx context.Context, userID, id uuid.UUID) (Task, error)
	List(ctx context.Context, userID uuid.UUID) ([]Task, error)
}
