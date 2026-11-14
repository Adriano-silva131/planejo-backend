package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Routine builds the user's week from the fixed tasks, which one week may
// move or skip, and the sessions the user places for the others, and plans
// what has no time from the progress of the week. Changes that take time are
// refused when they make more than domain.MaxLayers things happen at once.
type Routine struct {
	tasks    domain.Tasks
	repo     domain.Repository
	sessions domain.Sessions
	progress domain.Progress
	tx       Transactor
	now      func() time.Time
}

// Stores are where the routine reads and keeps its data.
type Stores struct {
	Tasks       domain.Tasks
	Adjustments domain.Repository
	Sessions    domain.Sessions
	Progress    domain.Progress
}

func NewRoutine(s Stores, tx Transactor) *Routine {
	return &Routine{
		tasks: s.Tasks, repo: s.Adjustments, sessions: s.Sessions, progress: s.Progress, tx: tx, now: time.Now,
	}
}

// WithClock makes r read the current time from now, so tests can fix what
// "today" is.
func (r *Routine) WithClock(now func() time.Time) *Routine {
	r.now = now
	return r
}

// Week builds and plans the week that contains day, as seen today in the time
// zone stored in ctx. Adjustments start on the Sunday before, whose overnight
// occurrences end in this week.
func (r *Routine) Week(ctx context.Context, userID uuid.UUID, day calendar.Date) (domain.Week, error) {
	monday := day.StartOfWeek()
	from, to := monday.AddDays(-1), monday.AddDays(6)
	fixed, plannable, err := r.tasks.List(ctx, userID)
	if err != nil {
		return domain.Week{}, err
	}
	adjustments, err := r.repo.List(ctx, userID, from, to)
	if err != nil {
		return domain.Week{}, err
	}
	sessions, err := r.sessions.List(ctx, userID, monday, to)
	if err != nil {
		return domain.Week{}, err
	}
	progress, err := r.weekProgress(ctx, userID, monday, plannable)
	if err != nil {
		return domain.Week{}, err
	}
	w := domain.BuildWeek(day, fixed, adjustments, plannable, sessions)
	w.Plan(calendar.DateOf(r.now().In(calendar.LocationFrom(ctx))), plannable, sessions, progress)
	return w, nil
}

func (r *Routine) weekProgress(
	ctx context.Context, userID uuid.UUID, monday calendar.Date, plannable []domain.PlannableTask,
) (domain.WeekProgress, error) {
	var p domain.WeekProgress
	var err error
	if p.Habits, err = r.progress.Habits(ctx, userID, monday); err != nil {
		return p, err
	}
	if p.CheckIns, err = r.progress.CheckIns(ctx, userID, monday, monday.AddDays(6)); err != nil {
		return p, err
	}
	var oneOffs []uuid.UUID
	for _, t := range plannable {
		if t.Kind == domain.OneOffKind {
			oneOffs = append(oneOffs, t.ID)
		}
	}
	if len(oneOffs) > 0 {
		p.Completed, err = r.progress.Completed(ctx, userID, oneOffs)
	}
	return p, err
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
	var saved domain.Adjustment
	err = r.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if saved, err = r.repo.Save(ctx, a); err != nil || saved.Skipped {
			return err
		}
		return r.checkDates(ctx, userID, taskID, date, date.AddDays(1))
	})
	if err != nil {
		return domain.Adjustment{}, err
	}
	return saved, nil
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
	return r.tx.InTx(ctx, func(ctx context.Context) error {
		if err := r.repo.Delete(ctx, userID, taskID, date); err != nil {
			return err
		}
		return r.checkDates(ctx, userID, taskID, date, date.AddDays(1))
	})
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
	var saved domain.Session
	err = r.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if saved, err = r.sessions.Save(ctx, s); err != nil {
			return err
		}
		return r.checkDates(ctx, userID, taskID, date)
	})
	if err != nil {
		return domain.Session{}, err
	}
	return saved, nil
}

func (r *Routine) RemoveSession(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	return r.sessions.Delete(ctx, userID, taskID, date)
}
