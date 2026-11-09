package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Routine builds the user's week from the fixed tasks, which one week may
// move or skip, and the sessions the user places for the others.
type Routine struct {
	tasks    domain.Tasks
	repo     domain.Repository
	sessions domain.Sessions
}

func NewRoutine(tasks domain.Tasks, repo domain.Repository, sessions domain.Sessions) *Routine {
	return &Routine{tasks: tasks, repo: repo, sessions: sessions}
}

// Week builds the week that contains day. Adjustments start on the Sunday
// before, whose overnight occurrences end in this week.
func (r *Routine) Week(ctx context.Context, userID uuid.UUID, day calendar.Date) (domain.Week, error) {
	monday := day.StartOfWeek()
	from, to := monday.AddDays(-1), monday.AddDays(6)
	tasks, err := r.tasks.Fixed(ctx, userID, from, to)
	if err != nil {
		return domain.Week{}, err
	}
	adjustments, err := r.repo.List(ctx, userID, from, to)
	if err != nil {
		return domain.Week{}, err
	}
	plannable, err := r.tasks.Plannable(ctx, userID)
	if err != nil {
		return domain.Week{}, err
	}
	sessions, err := r.sessions.List(ctx, userID, monday, to)
	if err != nil {
		return domain.Week{}, err
	}
	return domain.BuildWeek(day, tasks, adjustments, plannable, sessions), nil
}

// Adjust changes the occurrence of the task starting on date, replacing an
// earlier adjustment of it. The task itself is not changed.
func (r *Routine) Adjust(
	ctx context.Context, userID, taskID uuid.UUID, date calendar.Date, c domain.Change,
) (domain.Adjustment, error) {
	task, err := r.tasks.GetFixed(ctx, userID, taskID)
	if err != nil {
		return domain.Adjustment{}, err
	}
	a, err := domain.NewAdjustment(userID, task, date, c)
	if err != nil {
		return domain.Adjustment{}, err
	}
	return r.repo.Save(ctx, a)
}

// ScheduleChanged drops what no longer applies after the task's schedule
// changed. fixed is the task when it is fixed now: its sessions go, and so do
// the adjustments of dates that are no longer occurrences. Otherwise every
// adjustment goes, as the task has no occurrences anymore.
func (r *Routine) ScheduleChanged(ctx context.Context, userID, taskID uuid.UUID, fixed *domain.FixedTask) error {
	if fixed != nil {
		if err := r.sessions.DeleteForTask(ctx, userID, taskID); err != nil {
			return err
		}
	}
	adjustments, err := r.repo.ForTask(ctx, userID, taskID)
	if err != nil {
		return err
	}
	for _, a := range adjustments {
		if fixed == nil || !fixed.OccursOn(a.Date) {
			if err := r.repo.Delete(ctx, userID, taskID, a.Date); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reset brings the occurrence back to the task's pattern.
func (r *Routine) Reset(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	return r.repo.Delete(ctx, userID, taskID, date)
}

// PlaceSession sets aside time for a recurring or one-off task on date,
// replacing the session the task had that day.
func (r *Routine) PlaceSession(
	ctx context.Context, userID, taskID uuid.UUID, date calendar.Date, p domain.Placement,
) (domain.Session, error) {
	task, err := r.tasks.GetPlannable(ctx, userID, taskID)
	if err != nil {
		return domain.Session{}, err
	}
	s, err := domain.NewSession(userID, task, date, p)
	if err != nil {
		return domain.Session{}, err
	}
	return r.sessions.Save(ctx, s)
}

func (r *Routine) RemoveSession(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	return r.sessions.Delete(ctx, userID, taskID, date)
}
