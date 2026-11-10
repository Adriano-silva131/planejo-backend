package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Session is a time the user set aside for a recurring or one-off task on
// Date, from Start to End, on any day of the week. A task has at most one
// session a day, and a session ends by midnight: End is EndOfDay then.
type Session struct {
	UserID    uuid.UUID
	TaskID    uuid.UUID
	Date      calendar.Date
	Start     calendar.TimeOfDay
	End       calendar.TimeOfDay
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Placement is what the user asks for a session. Without End, a recurring
// session lasts the task's duration; a one-off task has no duration, so it
// needs End. An End of 00:00 is midnight.
type Placement struct {
	Start calendar.TimeOfDay
	End   calendar.OptionalTime
}

func NewSession(userID uuid.UUID, task PlannableTask, date calendar.Date, p Placement) (Session, error) {
	if date.IsZero() {
		return Session{}, calendar.ErrInvalidDate
	}
	if !p.Start.Valid() || !p.End.Valid() {
		return Session{}, calendar.ErrInvalidTime
	}
	end, err := sessionEnd(task, p)
	if err != nil {
		return Session{}, err
	}
	switch {
	case end == p.Start:
		return Session{}, ErrInvalidTimeRange
	case end < p.Start:
		return Session{}, ErrSessionPastMidnight
	}
	return Session{UserID: userID, TaskID: task.ID, Date: date, Start: p.Start, End: end}, nil
}

func sessionEnd(task PlannableTask, p Placement) (calendar.TimeOfDay, error) {
	if end, ok := p.End.Get(); ok {
		if end == 0 {
			return EndOfDay, nil
		}
		return end, nil
	}
	if task.Duration == 0 {
		return 0, ErrSessionEndRequired
	}
	end := p.Start + calendar.TimeOfDay(task.Duration/time.Minute)
	if end > EndOfDay {
		return 0, ErrSessionPastMidnight
	}
	return end, nil
}

// Sessions stores sessions, always scoped to one user.
type Sessions interface {
	// Save creates the session, or replaces the one of the same task and day.
	// It fails with ErrTaskNotFound when the task is not the user's, or was
	// deleted meanwhile.
	Save(ctx context.Context, s Session) (Session, error)
	Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error
	// List returns the sessions from from to to; a zero to has no end.
	List(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]Session, error)
	// DeleteForTask removes every session of the task.
	DeleteForTask(ctx context.Context, userID, taskID uuid.UUID) error
}
