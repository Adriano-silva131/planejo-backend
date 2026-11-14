package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

var (
	userID = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")
	// The week of Monday 2026-10-26.
	monday = calendar.NewDate(2026, time.October, 26)

	gym    = Task{ID: uuid.New(), Title: "Academia", Kind: RecurringTask, TimesPerWeek: 3}
	work   = Task{ID: uuid.New(), Title: "Trabalho", Kind: FixedTask}
	thesis = Task{ID: uuid.New(), Title: "TCC", Kind: OneOffTask, Deadline: monday.AddDays(40)}
)

func day(n int) calendar.Date {
	return monday.AddDays(n)
}

func TestNewCheckIn(t *testing.T) {
	c, err := NewCheckIn(userID, thesis, Entry{Date: day(1)}, day(3), nil)

	require.NoError(t, err)
	assert.Equal(t, CheckIn{UserID: userID, TaskID: thesis.ID, Date: day(1)}, c)
}

func TestNewCheckInKeepsTheStart(t *testing.T) {
	c, err := NewCheckIn(userID, gym, Entry{Date: day(1), Start: calendar.SomeTime(7 * 60)}, day(2), nil)

	require.NoError(t, err)
	assert.Equal(t, calendar.SomeTime(7*60), c.Start)
}

func TestNewCheckInAcceptsToday(t *testing.T) {
	_, err := NewCheckIn(userID, gym, Entry{Date: day(2)}, day(2), nil)

	assert.NoError(t, err)
}

func TestNewCheckInCompletesOneOffOnce(t *testing.T) {
	completed := []CheckIn{checkIn(thesis, day(1))}

	_, sameDay := NewCheckIn(userID, thesis, Entry{Date: day(1), Start: calendar.SomeTime(9 * 60)}, day(2), completed)
	_, otherDay := NewCheckIn(userID, thesis, Entry{Date: day(2)}, day(2), completed)
	_, recurring := NewCheckIn(userID, gym, Entry{Date: day(2)}, day(2), []CheckIn{checkIn(gym, day(1))})

	assert.NoError(t, sameDay, "the same day can be reported again")
	assert.ErrorIs(t, otherDay, ErrAlreadyCompleted)
	assert.NoError(t, recurring, "recurring tasks are done many times")
}

func TestNewCheckInRejects(t *testing.T) {
	cases := map[string]struct {
		task  Task
		entry Entry
		want  error
	}{
		"fixed task":    {work, Entry{Date: day(1)}, ErrFixedTaskCheckIn},
		"no date":       {gym, Entry{}, calendar.ErrInvalidDate},
		"future day":    {gym, Entry{Date: day(3)}, ErrFutureCheckIn},
		"invalid start": {gym, Entry{Date: day(1), Start: calendar.SomeTime(24 * 60)}, calendar.ErrInvalidTime},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewCheckIn(userID, tc.task, tc.entry, day(2), nil)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func checkIn(task Task, date calendar.Date) CheckIn {
	return CheckIn{UserID: userID, TaskID: task.ID, Date: date}
}

func TestSummarizeCountsRecurringDaysInTheWeek(t *testing.T) {
	// Set for Monday, Wednesday and Friday; Monday was swapped for Tuesday.
	checkIns := []CheckIn{
		checkIn(gym, day(-1)),
		checkIn(gym, day(1)),
		checkIn(gym, day(2)),
		checkIn(gym, day(4)),
		checkIn(gym, day(7)),
	}

	s := Summarize(day(3), []Task{gym, work}, checkIns)

	assert.Equal(t, monday, s.Start)
	assert.Equal(t, day(6), s.End)
	assert.Equal(t, []RecurringProgress{{Task: gym, Expected: 3, Done: []calendar.Date{day(1), day(2), day(4)}}}, s.Recurring)
	assert.Empty(t, s.OneOff, "fixed tasks are left out")
}

func TestSummarizeShowsOpenOneOff(t *testing.T) {
	s := Summarize(monday, []Task{thesis}, nil)

	assert.Equal(t, []OneOffProgress{{Task: thesis}}, s.OneOff)
}

func TestSummarizeShowsOneOffUntilTheWeekItIsCompleted(t *testing.T) {
	checkIns := []CheckIn{checkIn(thesis, day(2))}

	completedThisWeek := Summarize(monday, []Task{thesis}, checkIns)
	nextWeek := Summarize(day(7), []Task{thesis}, checkIns)
	previousWeek := Summarize(day(-7), []Task{thesis}, checkIns)

	require.Len(t, completedThisWeek.OneOff, 1)
	assert.Equal(t, day(2), completedThisWeek.OneOff[0].CompletedOn)
	assert.Empty(t, nextWeek.OneOff)
	require.Len(t, previousWeek.OneOff, 1)
	assert.True(t, previousWeek.OneOff[0].CompletedOn.IsZero(), "not completed yet in that week")
}

func TestSummarizeLeavesOutTasksCreatedAfterTheWeek(t *testing.T) {
	createdNextWeek := gym
	createdNextWeek.CreatedOn = day(7)
	createdOnSunday := thesis
	createdOnSunday.CreatedOn = day(6)

	s := Summarize(monday, []Task{createdNextWeek, createdOnSunday}, nil)

	assert.Empty(t, s.Recurring)
	assert.Equal(t, []OneOffProgress{{Task: createdOnSunday}}, s.OneOff)
}

func TestSummarizeWithoutCheckIns(t *testing.T) {
	s := Summarize(monday, []Task{gym, thesis}, nil)

	assert.Equal(t, []RecurringProgress{{Task: gym, Expected: 3}}, s.Recurring)
	assert.Equal(t, []OneOffProgress{{Task: thesis}}, s.OneOff)
}
