package domain

import (
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserID = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")

func weekdays(t *testing.T, days ...time.Weekday) calendar.Weekdays {
	t.Helper()
	w, err := calendar.NewWeekdays(days...)
	require.NoError(t, err)
	return w
}

func clock(t *testing.T, s string) calendar.TimeOfDay {
	t.Helper()
	tod, err := calendar.ParseTimeOfDay(s)
	require.NoError(t, err)
	return tod
}

func gym(t *testing.T) Details {
	t.Helper()
	return Details{
		Title:    "Academia",
		Area:     "Saúde",
		Priority: High,
		Schedule: RecurringSchedule{
			Weekdays: weekdays(t, time.Monday, time.Wednesday, time.Friday),
			Duration: time.Hour,
			Period:   Morning,
		},
	}
}

func work(t *testing.T) Details {
	t.Helper()
	return Details{
		Title: "Trabalho",
		Schedule: FixedSchedule{
			Weekdays: weekdays(t, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
			Start:    clock(t, "09:00"),
			End:      clock(t, "18:00"),
		},
	}
}

func report() Details {
	return Details{
		Title:    "Entregar relatório",
		Priority: Medium,
		Schedule: OneOffSchedule{Duration: 3 * time.Hour, Deadline: calendar.NewDate(2026, time.October, 30)},
	}
}

func TestNewAcceptsEachKind(t *testing.T) {
	for name, d := range map[string]Details{
		"fixed":     work(t),
		"recurring": gym(t),
		"one-off":   report(),
	} {
		t.Run(name, func(t *testing.T) {
			task, err := New(testUserID, d)

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, task.ID)
			assert.Equal(t, testUserID, task.UserID)
			assert.Equal(t, d, task.Details)
		})
	}
}

func TestNewTrimsTitleAndArea(t *testing.T) {
	d := gym(t)
	d.Title = "  Academia  "
	d.Area = " Saúde "

	task, err := New(testUserID, d)

	require.NoError(t, err)
	assert.Equal(t, "Academia", task.Title)
	assert.Equal(t, "Saúde", task.Area)
}

func TestNewAcceptsFixedTaskOnADate(t *testing.T) {
	d := work(t)
	d.Schedule = FixedSchedule{Date: calendar.NewDate(2026, time.November, 5), Start: clock(t, "15:00"), End: clock(t, "16:00")}

	_, err := New(testUserID, d)

	assert.NoError(t, err)
}

func TestNewRejectsInvalidDetails(t *testing.T) {
	cases := map[string]struct {
		change func(*Details)
		want   error
	}{
		"blank title":         {func(d *Details) { d.Title = "   " }, ErrTitleRequired},
		"long title":          {func(d *Details) { d.Title = strings.Repeat("a", MaxTitleLength+1) }, ErrTitleTooLong},
		"long area":           {func(d *Details) { d.Area = strings.Repeat("á", MaxAreaLength+1) }, ErrAreaTooLong},
		"no schedule":         {func(d *Details) { d.Schedule = nil }, ErrScheduleRequired},
		"no priority":         {func(d *Details) { d.Priority = "" }, ErrPriorityRequired},
		"unknown priority":    {func(d *Details) { d.Priority = "urgent" }, ErrInvalidPriority},
		"no weekdays":         {func(d *Details) { d.Schedule = RecurringSchedule{Duration: time.Hour} }, ErrWeekdaysRequired},
		"short session":       {setRecurringDuration(t, time.Minute), ErrInvalidSessionDuration},
		"long session":        {setRecurringDuration(t, 13*time.Hour), ErrInvalidSessionDuration},
		"unknown period":      {setRecurringPeriod(t, "dawn"), ErrInvalidPeriod},
		"one-off no deadline": {func(d *Details) { d.Schedule = OneOffSchedule{Duration: time.Hour} }, ErrDeadlineRequired},
		"one-off too long": {func(d *Details) {
			d.Schedule = OneOffSchedule{Duration: 101 * time.Hour, Deadline: calendar.NewDate(2026, time.October, 30)}
		}, ErrInvalidTotalDuration},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := gym(t)
			tc.change(&d)

			_, err := New(testUserID, d)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func setRecurringDuration(t *testing.T, duration time.Duration) func(*Details) {
	return func(d *Details) {
		d.Schedule = RecurringSchedule{Weekdays: weekdays(t, time.Monday), Duration: duration}
	}
}

func setRecurringPeriod(t *testing.T, period Period) func(*Details) {
	return func(d *Details) {
		d.Schedule = RecurringSchedule{Weekdays: weekdays(t, time.Monday), Duration: time.Hour, Period: period}
	}
}

func TestNewRejectsInvalidFixedTasks(t *testing.T) {
	mon := weekdays(t, time.Monday)
	day := calendar.NewDate(2026, time.November, 5)
	cases := map[string]struct {
		priority Priority
		schedule FixedSchedule
		want     error
	}{
		"with priority":     {High, FixedSchedule{Weekdays: mon, Start: clock(t, "09:00"), End: clock(t, "10:00")}, ErrFixedTaskPriority},
		"no days nor date":  {"", FixedSchedule{Start: clock(t, "09:00"), End: clock(t, "10:00")}, ErrFixedTaskDays},
		"days and date":     {"", FixedSchedule{Weekdays: mon, Date: day, Start: clock(t, "09:00"), End: clock(t, "10:00")}, ErrFixedTaskDays},
		"end before start":  {"", FixedSchedule{Weekdays: mon, Start: clock(t, "10:00"), End: clock(t, "09:00")}, ErrInvalidTimeRange},
		"empty range":       {"", FixedSchedule{Weekdays: mon, Start: clock(t, "10:00"), End: clock(t, "10:00")}, ErrInvalidTimeRange},
		"time out of range": {"", FixedSchedule{Weekdays: mon, Start: clock(t, "09:00"), End: 24 * 60}, calendar.ErrInvalidTime},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(testUserID, Details{Title: "Trabalho", Priority: tc.priority, Schedule: tc.schedule})

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestUpdateReplacesDetails(t *testing.T) {
	task, err := New(testUserID, gym(t))
	require.NoError(t, err)
	id := task.ID

	require.NoError(t, task.Update(report()))

	assert.Equal(t, id, task.ID)
	assert.Equal(t, report(), task.Details)
}

func TestUpdateKeepsTaskOnError(t *testing.T) {
	task, err := New(testUserID, gym(t))
	require.NoError(t, err)

	err = task.Update(Details{Title: ""})

	assert.ErrorIs(t, err, ErrTitleRequired)
	assert.Equal(t, gym(t), task.Details)
}
