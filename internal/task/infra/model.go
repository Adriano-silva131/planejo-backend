package infra

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// taskModel is the tasks row. Columns that only some kinds use are nullable.
type taskModel struct {
	ID              uuid.UUID `gorm:"primaryKey"`
	UserID          uuid.UUID
	Title           string
	Area            string
	Kind            string
	Priority        *string
	Weekdays        domain.Weekdays
	Date            *time.Time
	StartMinute     *int
	EndMinute       *int
	DurationMinutes *int64
	Period          *string
	Deadline        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (taskModel) TableName() string {
	return "tasks"
}

// editableColumns are written on update, including the ones set to NULL.
var editableColumns = []string{
	"title", "area", "kind", "priority", "weekdays", "date", "start_minute",
	"end_minute", "duration_minutes", "period", "deadline", "updated_at",
}

func toModel(t domain.Task) taskModel {
	m := taskModel{
		ID:        t.ID,
		UserID:    t.UserID,
		Title:     t.Title,
		Area:      t.Area,
		Kind:      string(t.Schedule.Kind()),
		Priority:  optional(string(t.Priority)),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	switch s := t.Schedule.(type) {
	case domain.FixedSchedule:
		m.Weekdays = s.Weekdays
		m.Date = optionalDate(s.Date)
		m.StartMinute = new(int(s.Start))
		m.EndMinute = new(int(s.End))
	case domain.RecurringSchedule:
		m.Weekdays = s.Weekdays
		m.DurationMinutes = new(int64(s.Duration / time.Minute))
		m.Period = optional(string(s.Period))
	case domain.OneOffSchedule:
		m.DurationMinutes = new(int64(s.Duration / time.Minute))
		m.Deadline = optionalDate(s.Deadline)
		m.Period = optional(string(s.Period))
	}
	return m
}

func toDomain(m taskModel) (domain.Task, error) {
	schedule, err := m.schedule()
	if err != nil {
		return domain.Task{}, err
	}
	return domain.Task{
		ID:     m.ID,
		UserID: m.UserID,
		Details: domain.Details{
			Title:    m.Title,
			Area:     m.Area,
			Priority: domain.Priority(value(m.Priority)),
			Schedule: schedule,
		},
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (m taskModel) schedule() (domain.Schedule, error) {
	switch domain.Kind(m.Kind) {
	case domain.Fixed:
		return domain.FixedSchedule{
			Weekdays: m.Weekdays,
			Date:     dateOf(m.Date),
			Start:    domain.TimeOfDay(value(m.StartMinute)),
			End:      domain.TimeOfDay(value(m.EndMinute)),
		}, nil
	case domain.Recurring:
		return domain.RecurringSchedule{
			Weekdays: m.Weekdays,
			Duration: minutes(m.DurationMinutes),
			Period:   domain.Period(value(m.Period)),
		}, nil
	case domain.OneOff:
		return domain.OneOffSchedule{
			Duration: minutes(m.DurationMinutes),
			Deadline: dateOf(m.Deadline),
			Period:   domain.Period(value(m.Period)),
		}, nil
	}
	return nil, fmt.Errorf("task %s: unknown kind %q", m.ID, m.Kind)
}

// optional maps the zero value to NULL.
func optional[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func optionalDate(d domain.Date) *time.Time {
	if d.IsZero() {
		return nil
	}
	return new(d.Time())
}

func dateOf(t *time.Time) domain.Date {
	if t == nil {
		return domain.Date{}
	}
	return domain.NewDate(t.Date())
}

func minutes(n *int64) time.Duration {
	return time.Duration(value(n)) * time.Minute
}
