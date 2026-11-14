package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

func TestNewSessionOnlyInAPeriod(t *testing.T) {
	gym, report := plannable()

	evening, err := NewSession(userID, gym, day(2), Placement{Period: calendar.Evening})
	require.NoError(t, err)
	morning, err := NewSession(userID, report, day(2), Placement{Period: calendar.Morning})
	require.NoError(t, err)

	assert.Equal(t, Session{UserID: userID, TaskID: gym.ID, Date: day(2), Period: calendar.Evening}, evening)
	assert.False(t, evening.Timed())
	assert.Equal(t, calendar.Morning, morning.Period, "a one-off task needs no end without a time")
}

func TestNewSessionRejectsATimeAndAPeriod(t *testing.T) {
	gym, _ := plannable()
	cases := map[string]struct {
		placement Placement
		want      error
	}{
		"period and start": {Placement{Period: calendar.Evening, Start: calendar.SomeTime(at(19))}, ErrSessionPeriodWithTime},
		"period and end":   {Placement{Period: calendar.Evening, End: calendar.SomeTime(at(20))}, ErrSessionPeriodWithTime},
		"neither":          {Placement{End: calendar.SomeTime(at(20))}, ErrSessionStartRequired},
		"unknown period":   {Placement{Period: "dawn"}, calendar.ErrInvalidPeriod},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewSession(userID, gym, day(0), tc.placement)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestBuildWeekLeavesSessionsInAPeriodOutOfTheBlocks(t *testing.T) {
	gym, report := plannable()
	sessions := []Session{
		{TaskID: gym.ID, Date: day(2), Period: calendar.Evening},
		{TaskID: report.ID, Date: day(2), Start: at(9), End: at(10)},
	}

	w := BuildWeek(monday, nil, nil, []PlannableTask{gym, report}, sessions)

	assert.Equal(t, []slot{{"Relatório", "09:00", "10:00"}}, slots(w.Days[2]))
}

func TestSessionsInAPeriodTakeNoLayer(t *testing.T) {
	work, lunch, _ := workday(t)
	english := PlannableTask{ID: uuid.New(), Title: "Inglês", Kind: RecurringKind, Duration: time.Hour}
	s := Schedule{
		Fixed:     []FixedTask{work, lunch},
		Plannable: []PlannableTask{english},
		Sessions:  []Session{{TaskID: english.ID, Date: day(0), Period: calendar.Afternoon}},
	}

	assert.NoError(t, s.CheckDates(english.ID, day(0)), "work and lunch are already two layers")
	assert.NoError(t, s.CheckWeekly(lunch.ID, monday))
}
