package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type Progress struct {
	tasks domain.Tasks
	repo  domain.Repository
	now   func() time.Time
}

func NewProgress(tasks domain.Tasks, repo domain.Repository) *Progress {
	return &Progress{tasks: tasks, repo: repo, now: time.Now}
}

// CheckIn marks the task as done on the entry's day, replacing what was
// reported for that day before. loc is the user's time zone, where "today" is.
func (p *Progress) CheckIn(
	ctx context.Context, userID, taskID uuid.UUID, e domain.Entry, loc *time.Location,
) (domain.CheckIn, error) {
	task, err := p.tasks.Get(ctx, userID, taskID)
	if err != nil {
		return domain.CheckIn{}, err
	}
	c, err := domain.NewCheckIn(userID, task, e, calendar.DateOf(p.now().In(loc)))
	if err != nil {
		return domain.CheckIn{}, err
	}
	return p.repo.Save(ctx, c)
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
	// One-off tasks add up every check-in until the end of the week.
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

// ScheduleChanged keeps the weekdays a task had when they change, so its past
// habit weeks are judged by them. Empty weekdays mean the task is not
// recurring; a change that keeps the weekdays is ignored. The day of the
// change is in the time zone stored in ctx, or UTC.
func (p *Progress) ScheduleChanged(ctx context.Context, userID, taskID uuid.UUID, before, after calendar.Weekdays) error {
	if before == after {
		return nil
	}
	changedOn := calendar.DateOf(p.now().In(calendar.LocationFrom(ctx)))
	return p.repo.SaveSchedule(ctx, userID, taskID, domain.PastSchedule{ChangedOn: changedOn, Weekdays: before})
}
