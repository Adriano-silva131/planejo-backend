package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Priority string

const (
	High   Priority = "high"
	Medium Priority = "medium"
	Low    Priority = "low"
)

const (
	MaxTitleLength = 120
	MaxAreaLength  = 50
)

// Details are the fields the user edits.
type Details struct {
	Title string
	// Area is a free label chosen by the user, such as "Saúde". Optional.
	Area string
	// Priority is required, except for fixed tasks, which always happen.
	Priority Priority
	Schedule Schedule
}

type Task struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Details
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(userID uuid.UUID, d Details) (Task, error) {
	d = d.normalized()
	if err := d.validate(); err != nil {
		return Task{}, err
	}
	return Task{ID: uuid.New(), UserID: userID, Details: d}, nil
}

// Update replaces the editable fields; on error the task is left unchanged.
func (t *Task) Update(d Details) error {
	d = d.normalized()
	if err := d.validate(); err != nil {
		return err
	}
	t.Details = d
	return nil
}

func (d Details) normalized() Details {
	d.Title = strings.TrimSpace(d.Title)
	d.Area = strings.TrimSpace(d.Area)
	return d
}

func (d Details) validate() error {
	switch {
	case d.Title == "":
		return ErrTitleRequired
	case utf8.RuneCountInString(d.Title) > MaxTitleLength:
		return ErrTitleTooLong
	case utf8.RuneCountInString(d.Area) > MaxAreaLength:
		return ErrAreaTooLong
	case d.Schedule == nil:
		return ErrScheduleRequired
	}
	if err := d.validatePriority(); err != nil {
		return err
	}
	return d.Schedule.validate()
}

func (d Details) validatePriority() error {
	if d.Schedule.Kind() == Fixed {
		if d.Priority != "" {
			return ErrFixedTaskPriority
		}
		return nil
	}
	switch d.Priority {
	case High, Medium, Low:
		return nil
	case "":
		return ErrPriorityRequired
	}
	return ErrInvalidPriority
}
