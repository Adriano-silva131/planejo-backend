package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	taskdomain "github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

func TestRecurringWeekdays(t *testing.T) {
	mon := calendar.Weekdays(1 << time.Monday)
	recurring := taskdomain.Task{Details: taskdomain.Details{
		Schedule: taskdomain.RecurringSchedule{Weekdays: mon, Duration: time.Hour},
	}}
	fixed := taskdomain.Task{Details: taskdomain.Details{
		Schedule: taskdomain.FixedSchedule{Weekdays: mon, Start: 540, End: 600},
	}}

	assert.Equal(t, mon, recurringWeekdays(recurring))
	assert.Zero(t, recurringWeekdays(fixed), "fixed tasks are no habits")
}
