package domain

import (
	"context"

	"github.com/google/uuid"
)

// AreaUsage is an area and how many of the user's tasks use it, written
// exactly as the user typed it.
type AreaUsage struct {
	Area  string
	Tasks int
}

// Repository stores tasks. Every method is scoped to one user: a task that
// belongs to someone else is reported as ErrTaskNotFound.
type Repository interface {
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Task, error)
	// List returns a page of the user's tasks, newest first, and the total.
	List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]Task, int, error)
	// All returns every task of the user, newest first.
	All(ctx context.Context, userID uuid.UUID) ([]Task, error)
	// Areas returns the areas the user has used, most used first.
	Areas(ctx context.Context, userID uuid.UUID) ([]AreaUsage, error)
	Update(ctx context.Context, t Task) (Task, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}
