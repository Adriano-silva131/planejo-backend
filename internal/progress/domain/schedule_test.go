package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

func TestScheduleInFollowsTheScheduleOfTheWeek(t *testing.T) {
	gym := gymHabit(t)
	before := gym.Weekdays
	gym.Weekdays, gym.TimesPerWeek = calendar.Weekdays(1<<time.Saturday), 1
	gym.CreatedOn = day(-14)
	gym.PastSchedules = []PastSchedule{{ChangedOn: day(2), Recurrence: Recurrence{TimesPerWeek: 3, Weekdays: before}}}

	last, ok := gym.ScheduleIn(day(-7))
	require.True(t, ok)
	this, ok := gym.ScheduleIn(monday)
	require.True(t, ok)
	next, ok := gym.ScheduleIn(day(7))
	require.True(t, ok)

	assert.Equal(t, WeekSchedule{Task: gym, Target: 3, Weekdays: before, Since: gym.CreatedOn}, last)
	assert.Equal(t, 1, this.Target, "the week of the change asks for the lower of old and new")
	assert.Equal(t, gym.Weekdays, next.Weekdays)
}

func TestScheduleInOnlyWhileRecurring(t *testing.T) {
	gym := gymHabit(t)
	gym.CreatedOn = day(3)
	report := Task{Kind: OneOffTask, CreatedOn: day(-30)}

	created, ok := gym.ScheduleIn(monday)
	_, before := gym.ScheduleIn(day(-7))
	_, oneOff := report.ScheduleIn(monday)

	require.True(t, ok)
	assert.Equal(t, 3, created.Target, "Thursday to Sunday still fit three days")
	assert.False(t, before, "the task did not exist yet")
	assert.False(t, oneOff)
}

func TestScheduleInStartsOnTheDayTheTaskBecameRecurring(t *testing.T) {
	gym := gymHabit(t)
	gym.CreatedOn = day(-30)
	gym.PastSchedules = []PastSchedule{
		{ChangedOn: day(-14), Recurrence: Recurrence{TimesPerWeek: 3}},
		{ChangedOn: day(4), Recurrence: Recurrence{}},
	}

	current, ok := gym.ScheduleIn(monday)
	require.True(t, ok)
	assert.Equal(t, day(4), current.Since)
	assert.Equal(t, 3, current.Target)

	next, ok := gym.ScheduleIn(day(7))
	require.True(t, ok)
	assert.Equal(t, day(4), next.Since)

	past, ok := gym.ScheduleIn(day(-21))
	require.True(t, ok)
	assert.Equal(t, gym.CreatedOn, past.Since, "a later conversion must not change an older week")
	_, inactive := gym.ScheduleIn(day(-7))
	assert.False(t, inactive)
}
