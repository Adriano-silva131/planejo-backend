package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// ScheduleListener is told that a task's schedule changed, inside the
// transaction of the change, with the task before and after it, so modules
// that keep data per occurrence can adapt.
type ScheduleListener interface {
	ScheduleChanged(ctx context.Context, userID uuid.UUID, before, after domain.Task) error
}

// Transactor runs fn in a transaction carried by ctx.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Tasks is the CRUD of a user's tasks.
type Tasks struct {
	repo      domain.Repository
	tx        Transactor
	listeners []ScheduleListener
}

func NewTasks(repo domain.Repository, tx Transactor) *Tasks {
	return &Tasks{repo: repo, tx: tx}
}

// OnScheduleChange registers l. It is not safe to call while serving requests.
func (s *Tasks) OnScheduleChange(l ScheduleListener) {
	s.listeners = append(s.listeners, l)
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

// Fixed returns the fixed tasks that may occur from from to to, for modules
// that build a period from them.
func (s *Tasks) Fixed(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]domain.Task, error) {
	return s.repo.Fixed(ctx, userID, from, to)
}

// Areas lists the areas the user has used, for the client to suggest.
func (s *Tasks) Areas(ctx context.Context, userID uuid.UUID) ([]domain.AreaUsage, error) {
	return s.repo.Areas(ctx, userID)
}

// Update replaces the task's details. When its schedule changes, the
// listeners are told in the same transaction.
func (s *Tasks) Update(ctx context.Context, userID, id uuid.UUID, d domain.Details) (domain.Task, error) {
	var updated domain.Task
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := s.repo.Get(ctx, userID, id)
		if err != nil {
			return err
		}
		before := task
		if err := task.Update(d); err != nil {
			return err
		}
		if updated, err = s.repo.Update(ctx, task); err != nil {
			return err
		}
		if updated.Schedule == before.Schedule {
			return nil
		}
		for _, l := range s.listeners {
			if err := l.ScheduleChanged(ctx, userID, before, updated); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Task{}, err
	}
	return updated, nil
}

func (s *Tasks) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}
