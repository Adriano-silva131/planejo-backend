package main

import (
	"context"

	"github.com/google/uuid"

	routinedomain "github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskapp "github.com/Adriano-silva131/planejo-backend/internal/task/app"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// routineTasks lets the routine module read tasks without importing the task
// module.
type routineTasks struct {
	tasks *taskapp.Tasks
}

var _ routinedomain.Tasks = routineTasks{}

func (r routineTasks) Fixed(
	ctx context.Context, userID uuid.UUID, from, to calendar.Date,
) ([]routinedomain.FixedTask, error) {
	tasks, err := r.tasks.Fixed(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	var fixed []routinedomain.FixedTask
	for _, t := range tasks {
		if f, ok := toFixedTask(t); ok {
			fixed = append(fixed, f)
		}
	}
	return fixed, nil
}

func (r routineTasks) Plannable(ctx context.Context, userID uuid.UUID) ([]routinedomain.PlannableTask, error) {
	tasks, err := r.tasks.All(ctx, userID)
	if err != nil {
		return nil, err
	}
	var plannable []routinedomain.PlannableTask
	for _, t := range tasks {
		if p, ok := toPlannableTask(t); ok {
			plannable = append(plannable, p)
		}
	}
	return plannable, nil
}

func (r routineTasks) GetFixed(ctx context.Context, userID, id uuid.UUID) (routinedomain.FixedTask, error) {
	t, err := r.tasks.Get(ctx, userID, id)
	if err != nil {
		return routinedomain.FixedTask{}, err
	}
	if f, ok := toFixedTask(t); ok {
		return f, nil
	}
	return routinedomain.FixedTask{}, routinedomain.ErrNoOccurrences
}

func (r routineTasks) GetPlannable(ctx context.Context, userID, id uuid.UUID) (routinedomain.PlannableTask, error) {
	t, err := r.tasks.Get(ctx, userID, id)
	if err != nil {
		return routinedomain.PlannableTask{}, err
	}
	if p, ok := toPlannableTask(t); ok {
		return p, nil
	}
	return routinedomain.PlannableTask{}, routinedomain.ErrFixedTaskSession
}

func toFixedTask(t taskdomain.Task) (routinedomain.FixedTask, bool) {
	s, ok := t.Schedule.(taskdomain.FixedSchedule)
	if !ok {
		return routinedomain.FixedTask{}, false
	}
	return routinedomain.FixedTask{
		ID:       t.ID,
		Title:    t.Title,
		Area:     t.Area,
		Weekdays: s.Weekdays,
		Date:     s.Date,
		Start:    s.Start,
		End:      s.End,

		CreatedOn: calendar.DateOf(t.CreatedAt.UTC()),
	}, true
}

// toPlannableTask maps a recurring or one-off task.
func toPlannableTask(t taskdomain.Task) (routinedomain.PlannableTask, bool) {
	p := routinedomain.PlannableTask{ID: t.ID, Title: t.Title, Area: t.Area}
	switch s := t.Schedule.(type) {
	case taskdomain.RecurringSchedule:
		p.Kind, p.Duration = routinedomain.RecurringKind, s.Duration
	case taskdomain.OneOffSchedule:
		p.Kind = routinedomain.OneOffKind
	default:
		return routinedomain.PlannableTask{}, false
	}
	return p, true
}
