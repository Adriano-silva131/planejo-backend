package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository stores tasks. Every method is scoped to one user: a task that
// belongs to someone else is reported as ErrTaskNotFound.
type Repository interface {
	Create(ctx context.Context, t Task) (Task, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Task, error)
	// List returns a page of the user's tasks, newest first, and the total.
	List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]Task, int, error)
	// All returns every task of the user, newest first.
	All(ctx context.Context, userID uuid.UUID) ([]Task, error)
	Update(ctx context.Context, t Task) (Task, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}
