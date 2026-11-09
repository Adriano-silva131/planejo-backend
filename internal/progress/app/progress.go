package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Transactor runs fn in a transaction carried by ctx.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Progress struct {
	tasks domain.Tasks
	repo  domain.Repository
	tx    Transactor
	now   func() time.Time
}

func NewProgress(tasks domain.Tasks, repo domain.Repository, tx Transactor) *Progress {
	return &Progress{tasks: tasks, repo: repo, tx: tx, now: time.Now}
}

// CheckIn marks the task as done on the entry's day, replacing what was
// reported for that day before. loc is the user's time zone, where "today" is.
// The task is locked meanwhile, so a one-off is not completed on two days at
// once.
func (p *Progress) CheckIn(
	ctx context.Context, userID, taskID uuid.UUID, e domain.Entry, loc *time.Location,
) (domain.CheckIn, error) {
	var saved domain.CheckIn
	err := p.tx.InTx(ctx, func(ctx context.Context) error {
		if err := p.repo.LockTask(ctx, userID, taskID); err != nil {
			return err
		}
		c, err := p.newCheckIn(ctx, userID, taskID, e, calendar.DateOf(p.now().In(loc)))
		if err != nil {
			return err
		}
		saved, err = p.repo.Save(ctx, c)
		return err
	})
	if err != nil {
		return domain.CheckIn{}, err
	}
	return saved, nil
}

func (p *Progress) newCheckIn(
	ctx context.Context, userID, taskID uuid.UUID, e domain.Entry, today calendar.Date,
) (domain.CheckIn, error) {
	task, err := p.tasks.Get(ctx, userID, taskID)
	if err != nil {
		return domain.CheckIn{}, err
	}
	var existing []domain.CheckIn
	if task.Kind == domain.OneOffTask {
		if existing, err = p.repo.List(ctx, userID, domain.Filter{TaskID: taskID}); err != nil {
			return domain.CheckIn{}, err
		}
	}
	return domain.NewCheckIn(userID, task, e, today, existing)
}

func (p *Progress) Undo(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	return p.repo.Delete(ctx, userID, taskID, date)
}

// History lists the check-ins of the task in f.TaskID within f's dates.
func (p *Progress) History(ctx context.Context, userID uuid.UUID, f domain.Filter) ([]domain.CheckIn, error) {
	if _, err := p.tasks.Get(ctx, userID, f.TaskID); err != nil {
		return nil, err
	}
	return p.repo.List(ctx, userID, f)
}

// Week summarizes the week that contains day.
func (p *Progress) Week(ctx context.Context, userID uuid.UUID, day calendar.Date) (domain.WeekSummary, error) {
	tasks, err := p.tasks.List(ctx, userID)
	if err != nil {
		return domain.WeekSummary{}, err
	}
	// A one-off task completed before the week is left out.
	checkIns, err := p.repo.List(ctx, userID, domain.Filter{To: day.StartOfWeek().AddDays(6)})
	if err != nil {
		return domain.WeekSummary{}, err
	}
	return domain.Summarize(day, tasks, checkIns), nil
}

// Habits follows the recurring tasks over the weeks weeks up to the one that
// contains day. loc is the user's time zone, which tells today and the day
// each task was created.
func (p *Progress) Habits(
	ctx context.Context, userID uuid.UUID, day calendar.Date, weeks int, loc *time.Location,
) ([]domain.Habit, error) {
	tasks, err := p.tasks.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	schedules, err := p.repo.Schedules(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		t := &tasks[i]
		if !t.CreatedAt.IsZero() {
			t.CreatedOn = calendar.DateOf(t.CreatedAt.In(loc))
		}
		t.PastSchedules = schedules[t.ID]
	}
	today := calendar.DateOf(p.now().In(loc))
	ids, from := habitScope(tasks)
	if len(ids) == 0 {
		return domain.BuildHabits(today, day, weeks, tasks, nil), nil
	}
	checkIns, err := p.repo.List(ctx, userID, domain.Filter{TaskIDs: ids, From: from, To: day.StartOfWeek().AddDays(6)})
	if err != nil {
		return nil, err
	}
	return domain.BuildHabits(today, day, weeks, tasks, checkIns), nil
}

// habitScope returns the recurring tasks and the Monday of the oldest one's
// creation week: no grid or streak needs other check-ins. A zero from means
// no bound.
func habitScope(tasks []domain.Task) (ids []uuid.UUID, from calendar.Date) {
	for _, t := range tasks {
		if t.Kind != domain.RecurringTask {
			continue
		}
		ids = append(ids, t.ID)
		if !t.CreatedOn.IsZero() && (from.IsZero() || t.CreatedOn.Before(from)) {
			from = t.CreatedOn
		}
	}
	if from.IsZero() {
		return ids, from
	}
	return ids, from.StartOfWeek()
}

// ScheduleChanged keeps how a task repeated when that changes, so its past
// habit weeks are judged by it. A change that keeps the recurrence is
// ignored. The day of the change is in the time zone stored in ctx, or UTC.
func (p *Progress) ScheduleChanged(ctx context.Context, userID, taskID uuid.UUID, before, after domain.Recurrence) error {
	if before == after {
		return nil
	}
	changedOn := calendar.DateOf(p.now().In(calendar.LocationFrom(ctx)))
	return p.repo.SaveSchedule(ctx, userID, taskID, domain.PastSchedule{ChangedOn: changedOn, Recurrence: before})
}

// BecameOneOff drops the check-ins of a task that is now one-off: they were
// done before, and its only check-in is the day it is completed.
func (p *Progress) BecameOneOff(ctx context.Context, userID, taskID uuid.UUID) error {
	return p.repo.DeleteForTask(ctx, userID, taskID)
}
