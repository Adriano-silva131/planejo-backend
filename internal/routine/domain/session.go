package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// Session is a time the user set aside for a recurring or one-off task on
// Date, on any day of the week: from Start to End, or only in Period, a part
// of the day, without a time. A task has at most one session a day, and a
// session ends by midnight: End is EndOfDay then.
type Session struct {
	UserID    uuid.UUID
	TaskID    uuid.UUID
	Date      calendar.Date
	Start     calendar.TimeOfDay
	End       calendar.TimeOfDay
	Period    calendar.Period
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Timed reports whether the session has a time; otherwise it has a Period,
// and Start and End are zero.
func (s Session) Timed() bool {
	return s.Period == ""
}

// Placement is what the user asks for a session: a Start, or a Period without
// Start and End. Without End, a recurring session lasts the task's duration;
// a one-off task has no duration, so it needs End. An End of 00:00 is
// midnight.
type Placement struct {
	Start  calendar.OptionalTime
	End    calendar.OptionalTime
	Period calendar.Period
}

func NewSession(userID uuid.UUID, task PlannableTask, date calendar.Date, p Placement) (Session, error) {
	if date.IsZero() {
		return Session{}, calendar.ErrInvalidDate
	}
	s := Session{UserID: userID, TaskID: task.ID, Date: date}
	if p.Period != "" {
		return s.inPeriod(p)
	}
	start, ok := p.Start.Get()
	switch {
	case !ok:
		return Session{}, ErrSessionStartRequired
	case !start.Valid() || !p.End.Valid():
		return Session{}, calendar.ErrInvalidTime
	}
	end, err := sessionEnd(task, start, p.End)
	if err != nil {
		return Session{}, err
	}
	switch {
	case end == start:
		return Session{}, ErrInvalidTimeRange
	case end < start:
		return Session{}, ErrSessionPastMidnight
	}
	s.Start, s.End = start, end
	return s, nil
}

func (s Session) inPeriod(p Placement) (Session, error) {
	_, hasStart := p.Start.Get()
	_, hasEnd := p.End.Get()
	switch {
	case hasStart || hasEnd:
		return Session{}, ErrSessionPeriodWithTime
	case !p.Period.Valid():
		return Session{}, calendar.ErrInvalidPeriod
	}
	s.Period = p.Period
	return s, nil
}

func sessionEnd(task PlannableTask, start calendar.TimeOfDay, optionalEnd calendar.OptionalTime) (calendar.TimeOfDay, error) {
	if end, ok := optionalEnd.Get(); ok {
		if end == 0 {
			return EndOfDay, nil
		}
		return end, nil
	}
	if task.Duration == 0 {
		return 0, ErrSessionEndRequired
	}
	end := start + calendar.TimeOfDay(task.Duration/time.Minute)
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
