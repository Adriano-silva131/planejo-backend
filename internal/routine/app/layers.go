package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Transactor runs fn in a transaction carried by ctx.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// CheckFixed fails with a schedule_overlap error when the fixed task, as
// saved in the transaction in ctx, makes more than domain.MaxLayers things
// happen at once. A weekly task is checked from the current week on, in the
// time zone stored in ctx.
func (r *Routine) CheckFixed(ctx context.Context, userID uuid.UUID, task domain.FixedTask) error {
	if !task.Date.IsZero() {
		return r.checkDates(ctx, userID, task.ID, task.Date, task.Date.AddDays(1))
	}
	monday := calendar.DateOf(r.now().In(calendar.LocationFrom(ctx))).StartOfWeek()
	s, err := r.schedule(ctx, userID, monday, calendar.Date{})
	if err != nil {
		return err
	}
	return s.CheckWeekly(task.ID, monday)
}

// checkDates checks the blocks of the task on dates, which are in order.
func (r *Routine) checkDates(ctx context.Context, userID, taskID uuid.UUID, dates ...calendar.Date) error {
	from := dates[0].StartOfWeek()
	to := dates[len(dates)-1].StartOfWeek().AddDays(6)
	s, err := r.schedule(ctx, userID, from, to)
	if err != nil {
		return err
	}
	return s.CheckDates(taskID, dates...)
}

// schedule locks the user's routine until the transaction ends, so changes
// made at the same time are checked one after the other, and loads it from
// the Monday from to to; a zero to has no end.
func (r *Routine) schedule(ctx context.Context, userID uuid.UUID, from, to calendar.Date) (domain.Schedule, error) {
	if err := r.repo.Lock(ctx, userID); err != nil {
		return domain.Schedule{}, err
	}
	fixed, plannable, err := r.tasks.List(ctx, userID)
	if err != nil {
		return domain.Schedule{}, err
	}
	// From the Sunday before, whose overnight occurrences end on Monday.
	adjustments, err := r.repo.List(ctx, userID, from.AddDays(-1), to)
	if err != nil {
		return domain.Schedule{}, err
	}
	sessions, err := r.sessions.List(ctx, userID, from, to)
	if err != nil {
		return domain.Schedule{}, err
	}
	return domain.Schedule{Fixed: fixed, Plannable: plannable, Adjustments: adjustments, Sessions: sessions}, nil
}
