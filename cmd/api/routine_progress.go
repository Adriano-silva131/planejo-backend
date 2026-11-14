package main

import (
	"context"

	"github.com/google/uuid"

	progressapp "github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	progressdomain "github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	routinedomain "github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// routineProgress lets the routine module read the progress of a week
// without importing the progress module.
type routineProgress struct {
	progress *progressapp.Progress
}

var _ routinedomain.Progress = routineProgress{}

func (r routineProgress) Habits(ctx context.Context, userID uuid.UUID, monday calendar.Date) ([]routinedomain.Habit, error) {
	schedules, err := r.progress.WeekSchedules(ctx, userID, monday, calendar.LocationFrom(ctx))
	if err != nil {
		return nil, err
	}
	habits := make([]routinedomain.Habit, 0, len(schedules))
	for _, s := range schedules {
		habits = append(habits, routinedomain.Habit{
			TaskID: s.Task.ID, Target: s.Target, Weekdays: s.Weekdays, Since: s.Task.CreatedOn,
		})
	}
	return habits, nil
}

func (r routineProgress) CheckIns(
	ctx context.Context, userID uuid.UUID, from, to calendar.Date,
) ([]routinedomain.CheckIn, error) {
	checkIns, err := r.progress.CheckIns(ctx, userID, progressdomain.Filter{From: from, To: to})
	if err != nil {
		return nil, err
	}
	out := make([]routinedomain.CheckIn, 0, len(checkIns))
	for _, c := range checkIns {
		out = append(out, routinedomain.CheckIn{TaskID: c.TaskID, Date: c.Date, Start: c.Start})
	}
	return out, nil
}

func (r routineProgress) Completed(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	checkIns, err := r.progress.CheckIns(ctx, userID, progressdomain.Filter{TaskIDs: ids})
	if err != nil {
		return nil, err
	}
	completed := make(map[uuid.UUID]bool, len(checkIns))
	for _, c := range checkIns {
		completed[c.TaskID] = true
	}
	return completed, nil
}
