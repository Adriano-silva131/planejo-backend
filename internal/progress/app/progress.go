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
	tasks, checkIns, err := p.untilEndOfWeek(ctx, userID, day)
	if err != nil {
		return domain.WeekSummary{}, err
	}
	return domain.Summarize(day, tasks, checkIns), nil
}

// Habits follows the recurring tasks over the weeks weeks up to the one that
// contains day.
func (p *Progress) Habits(ctx context.Context, userID uuid.UUID, day calendar.Date, weeks int) ([]domain.Habit, error) {
	tasks, checkIns, err := p.untilEndOfWeek(ctx, userID, day)
	if err != nil {
		return nil, err
	}
	return domain.BuildHabits(day, weeks, tasks, checkIns), nil
}

// untilEndOfWeek loads the user's tasks and every check-in until the end of
// the week that contains day: one-off totals and streaks need the history.
func (p *Progress) untilEndOfWeek(
	ctx context.Context, userID uuid.UUID, day calendar.Date,
) ([]domain.Task, []domain.CheckIn, error) {
	tasks, err := p.tasks.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	checkIns, err := p.repo.List(ctx, userID, domain.Filter{To: day.StartOfWeek().AddDays(6)})
	if err != nil {
		return nil, nil, err
	}
	return tasks, checkIns, nil
}
