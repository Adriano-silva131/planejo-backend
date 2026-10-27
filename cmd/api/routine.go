package main

import (
	"context"

	"github.com/google/uuid"

	routinedomain "github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	taskapp "github.com/Adriano-silva131/planejo-backend/internal/task/app"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// routineTasks lets the routine module read fixed tasks without importing the
// task module.
type routineTasks struct {
	tasks *taskapp.Tasks
}

var _ routinedomain.Tasks = routineTasks{}

func (r routineTasks) Fixed(ctx context.Context, userID uuid.UUID) ([]routinedomain.FixedTask, error) {
	tasks, err := r.tasks.All(ctx, userID)
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

func (r routineTasks) Get(ctx context.Context, userID, id uuid.UUID) (routinedomain.FixedTask, error) {
	t, err := r.tasks.Get(ctx, userID, id)
	if err != nil {
		return routinedomain.FixedTask{}, err
	}
	f, ok := toFixedTask(t)
	if !ok {
		return routinedomain.FixedTask{}, routinedomain.ErrNotFixedTask
	}
	return f, nil
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
	}, true
}
