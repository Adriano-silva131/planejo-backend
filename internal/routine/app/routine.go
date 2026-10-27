package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Routine builds the user's week from the fixed tasks and lets one week
// differ from the pattern.
type Routine struct {
	tasks domain.Tasks
	repo  domain.Repository
}

func NewRoutine(tasks domain.Tasks, repo domain.Repository) *Routine {
	return &Routine{tasks: tasks, repo: repo}
}

// Week builds the week that contains day. Adjustments start on the Sunday
// before, whose overnight occurrences end in this week.
func (r *Routine) Week(ctx context.Context, userID uuid.UUID, day calendar.Date) (domain.Week, error) {
	tasks, err := r.tasks.Fixed(ctx, userID)
	if err != nil {
		return domain.Week{}, err
	}
	monday := day.StartOfWeek()
	adjustments, err := r.repo.List(ctx, userID, monday.AddDays(-1), monday.AddDays(6))
	if err != nil {
		return domain.Week{}, err
	}
	return domain.BuildWeek(day, tasks, adjustments), nil
}

// Adjust changes the occurrence of the task starting on date, replacing an
// earlier adjustment of it. The task itself is not changed.
func (r *Routine) Adjust(
	ctx context.Context, userID, taskID uuid.UUID, date calendar.Date, c domain.Change,
) (domain.Adjustment, error) {
	task, err := r.tasks.Get(ctx, userID, taskID)
	if err != nil {
		return domain.Adjustment{}, err
	}
	a, err := domain.NewAdjustment(userID, task, date, c)
	if err != nil {
		return domain.Adjustment{}, err
	}
	return r.repo.Save(ctx, a)
}

// Reset brings the occurrence back to the task's pattern.
func (r *Routine) Reset(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	return r.repo.Delete(ctx, userID, taskID, date)
}
