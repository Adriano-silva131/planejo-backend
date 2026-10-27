package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// Tasks is the CRUD of a user's tasks.
type Tasks struct {
	repo domain.Repository
}

func NewTasks(repo domain.Repository) *Tasks {
	return &Tasks{repo: repo}
}

func (s *Tasks) Create(ctx context.Context, userID uuid.UUID, d domain.Details) (domain.Task, error) {
	task, err := domain.New(userID, d)
	if err != nil {
		return domain.Task{}, err
	}
	return s.repo.Create(ctx, task)
}

func (s *Tasks) Get(ctx context.Context, userID, id uuid.UUID) (domain.Task, error) {
	return s.repo.Get(ctx, userID, id)
}

// List returns page number page (from 1) with perPage tasks, and the total.
func (s *Tasks) List(ctx context.Context, userID uuid.UUID, page, perPage int) ([]domain.Task, int, error) {
	return s.repo.List(ctx, userID, (page-1)*perPage, perPage)
}

// All returns every task of the user, for modules that need the whole set.
func (s *Tasks) All(ctx context.Context, userID uuid.UUID) ([]domain.Task, error) {
	return s.repo.All(ctx, userID)
}

// Areas lists the areas the user has used, for the client to suggest.
func (s *Tasks) Areas(ctx context.Context, userID uuid.UUID) ([]domain.AreaUsage, error) {
	return s.repo.Areas(ctx, userID)
}

func (s *Tasks) Update(ctx context.Context, userID, id uuid.UUID, d domain.Details) (domain.Task, error) {
	task, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return domain.Task{}, err
	}
	if err := task.Update(d); err != nil {
		return domain.Task{}, err
	}
	return s.repo.Update(ctx, task)
}

func (s *Tasks) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}
