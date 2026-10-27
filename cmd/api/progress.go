package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	progressdomain "github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	taskapp "github.com/Adriano-silva131/planejo-backend/internal/task/app"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// progressTasks lets the progress module read tasks without importing the
// task module.
type progressTasks struct {
	tasks *taskapp.Tasks
}

var _ progressdomain.Tasks = progressTasks{}

func (p progressTasks) Get(ctx context.Context, userID, id uuid.UUID) (progressdomain.Task, error) {
	t, err := p.tasks.Get(ctx, userID, id)
	if err != nil {
		return progressdomain.Task{}, err
	}
	return toProgressTask(t), nil
}

func (p progressTasks) List(ctx context.Context, userID uuid.UUID) ([]progressdomain.Task, error) {
	tasks, err := p.tasks.All(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]progressdomain.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toProgressTask(t))
	}
	return out, nil
}

func toProgressTask(t taskdomain.Task) progressdomain.Task {
	p := progressdomain.Task{ID: t.ID, Title: t.Title}
	switch s := t.Schedule.(type) {
	case taskdomain.FixedSchedule:
		p.Kind = progressdomain.FixedTask
	case taskdomain.RecurringSchedule:
		p.Kind = progressdomain.RecurringTask
		p.TimesPerWeek = len(s.Weekdays.Days())
	case taskdomain.OneOffSchedule:
		p.Kind = progressdomain.OneOffTask
		p.TotalMinutes = int(s.Duration / time.Minute)
		p.Deadline = s.Deadline
	}
	return p
}
