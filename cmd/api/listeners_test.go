package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	progressdomain "github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

func TestRecurrence(t *testing.T) {
	mon := calendar.Weekdays(1 << time.Monday)
	recurring := taskdomain.Task{Details: taskdomain.Details{
		Schedule: taskdomain.RecurringSchedule{TimesPerWeek: 3, Weekdays: mon, Duration: time.Hour},
	}}
	fixed := taskdomain.Task{Details: taskdomain.Details{
		Schedule: taskdomain.FixedSchedule{Weekdays: mon, Start: 540, End: 600},
	}}

	assert.Equal(t, progressdomain.Recurrence{TimesPerWeek: 3, Weekdays: mon}, recurrence(recurring))
	assert.Zero(t, recurrence(fixed), "fixed tasks are no habits")
}
