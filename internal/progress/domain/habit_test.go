package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// gymHabit is set for Monday, Wednesday and Friday; checks are on days
// relative to the Monday of the reference week.
func gymHabit(t *testing.T) Task {
	t.Helper()
	days, err := calendar.NewWeekdays(time.Monday, time.Wednesday, time.Friday)
	require.NoError(t, err)
	return Task{ID: uuid.New(), Title: "Academia", Area: "Saúde", Kind: RecurringTask, Weekdays: days, TimesPerWeek: 3}
}

func checks(task Task, days ...int) []CheckIn {
	out := make([]CheckIn, 0, len(days))
	for _, d := range days {
		out = append(out, checkIn(task, day(d), 0, false))
	}
	return out
}

func TestHabitGridAndSwappedDay(t *testing.T) {
	gym := gymHabit(t)
	// Last week on the set days; this week Tuesday instead of Monday, and Wednesday.
	cs := checks(gym, -7, -5, -3, 1, 2)

	habits := BuildHabits(day(2), 2, []Task{gym}, cs)

	require.Len(t, habits, 1)
	h := habits[0]
	require.Len(t, h.Weeks, 2)
	assert.Equal(t, day(-7), h.Weeks[0].Start)
	assert.True(t, h.Weeks[0].Met)
	assert.Equal(t, 2, h.Weeks[1].Done)
	assert.False(t, h.Weeks[1].Met)
	assert.Equal(t, HabitDay{Date: day(0), Scheduled: true, Done: false}, h.Weeks[1].Days[0])
	assert.Equal(t, HabitDay{Date: day(1), Scheduled: false, Done: true}, h.Weeks[1].Days[1])
	assert.Equal(t, 1, h.Streak, "the week in progress does not break the streak")
}

func TestHabitStreak(t *testing.T) {
	gym := gymHabit(t)
	cases := map[string]struct {
		days []int
		want int
	}{
		"current week met counts":          {[]int{-7, -5, -3, 0, 1, 4}, 2},
		"past week not met resets":         {[]int{-14, -12, -10, -7, 0, 2, 4}, 1},
		"nothing yet":                      {nil, 0},
		"only the current week, in course": {[]int{0}, 0},
		"goes beyond the grid":             {[]int{-35, -33, -31, -28, -26, -24, -21, -19, -17, -14, -12, -10, -7, -5, -3}, 5},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := BuildHabits(day(2), 2, []Task{gym}, checks(gym, tc.days...))[0]

			assert.Equal(t, tc.want, h.Streak)
		})
	}
}

func TestHabitStartsOnTheCreationWeek(t *testing.T) {
	gym := gymHabit(t)
	gym.CreatedOn = day(-5) // Wednesday of last week
	cs := checks(gym, -14, -12, -10, -7, -5, -3)

	h := BuildHabits(day(2), 4, []Task{gym}, cs)[0]

	require.Len(t, h.Weeks, 2, "no weeks before the task existed")
	assert.Equal(t, day(-7), h.Weeks[0].Start)
	assert.Equal(t, 1, h.Streak, "older weeks do not count")
}

func TestHabitsOnlyForRecurringTasksThatExist(t *testing.T) {
	gym := gymHabit(t)
	later := gymHabit(t)
	later.CreatedOn = day(7)

	habits := BuildHabits(day(2), 2, []Task{work, thesis, later, gym}, nil)

	require.Len(t, habits, 1)
	assert.Equal(t, gym.ID, habits[0].Task.ID)
	assert.Len(t, habits[0].Weeks[1].Days, 7)
}
