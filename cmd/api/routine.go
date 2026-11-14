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

func (r routineTasks) List(
	ctx context.Context, userID uuid.UUID,
) ([]routinedomain.FixedTask, []routinedomain.PlannableTask, error) {
	tasks, err := r.tasks.All(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	var fixed []routinedomain.FixedTask
	var plannable []routinedomain.PlannableTask
	for _, t := range tasks {
		if f, ok := toFixedTask(t); ok {
			fixed = append(fixed, f)
		} else if p, ok := toPlannableTask(ctx, t); ok {
			plannable = append(plannable, p)
		}
	}
	return fixed, plannable, nil
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
	if p, ok := toPlannableTask(ctx, t); ok {
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
func toPlannableTask(ctx context.Context, t taskdomain.Task) (routinedomain.PlannableTask, bool) {
	p := routinedomain.PlannableTask{ID: t.ID, Title: t.Title, Area: t.Area}
	if !t.CreatedAt.IsZero() {
		p.CreatedOn = calendar.DateOf(t.CreatedAt.In(calendar.LocationFrom(ctx)))
	}
	switch s := t.Schedule.(type) {
	case taskdomain.RecurringSchedule:
		p.Kind, p.Duration, p.Period = routinedomain.RecurringKind, s.Duration, s.Period
	case taskdomain.OneOffSchedule:
		p.Kind, p.Period, p.Deadline = routinedomain.OneOffKind, s.Period, s.Deadline
	default:
		return routinedomain.PlannableTask{}, false
	}
	return p, true
}
