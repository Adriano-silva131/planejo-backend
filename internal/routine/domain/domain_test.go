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
)

func day(n int) calendar.Date {
	return monday.AddDays(n)
}

// at returns the time at hour h o'clock.
func at(h int) calendar.TimeOfDay {
	return calendar.TimeOfDay(h * 60)
}

func weekdays(t *testing.T, days ...time.Weekday) calendar.Weekdays {
	t.Helper()
	w, err := calendar.NewWeekdays(days...)
	require.NoError(t, err)
	return w
}

func fixtures(t *testing.T) (work, class, lunch, doctor, sleep FixedTask) {
	t.Helper()
	weekdaysOnly := weekdays(t, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
	work = FixedTask{ID: uuid.New(), Title: "Trabalho", Area: "Trabalho", Weekdays: weekdaysOnly, Start: at(9), End: at(18)}
	class = FixedTask{ID: uuid.New(), Title: "Faculdade", Area: "Estudos", Weekdays: weekdays(t, time.Monday), Start: at(19), End: at(22)}
	lunch = FixedTask{ID: uuid.New(), Title: "Almoço", Area: "Alimentação", Weekdays: weekdaysOnly, Start: at(12), End: at(13)}
	doctor = FixedTask{ID: uuid.New(), Title: "Consulta", Area: "Saúde", Date: day(3), Start: at(15), End: at(16)}
	sleep = FixedTask{ID: uuid.New(), Title: "Dormir", Area: "Descanso", Weekdays: weekdays(t, time.Sunday), Start: at(23), End: at(7)}
	return work, class, lunch, doctor, sleep
}

type slot struct {
	title      string
	start, end string
}

func slots(d Day) []slot {
	out := []slot{}
	for _, b := range d.Blocks {
		out = append(out, slot{b.Title, b.Start.String(), b.End.String()})
	}
	return out
}

func TestBuildWeekFromThePattern(t *testing.T) {
	work, class, lunch, doctor, _ := fixtures(t)

	w := BuildWeek(day(2), []FixedTask{class, work, lunch, doctor}, nil)

	assert.Equal(t, monday, w.Start)
	assert.Equal(t, day(6), w.End)
	require.Len(t, w.Days, 7)
	assert.Equal(t, []slot{{"Trabalho", "09:00", "18:00"}, {"Almoço", "12:00", "13:00"}, {"Faculdade", "19:00", "22:00"}}, slots(w.Days[0]))
	assert.Equal(t, []slot{{"Trabalho", "09:00", "18:00"}, {"Almoço", "12:00", "13:00"}}, slots(w.Days[1]))
	assert.Equal(t, []slot{{"Trabalho", "09:00", "18:00"}, {"Almoço", "12:00", "13:00"}, {"Consulta", "15:00", "16:00"}}, slots(w.Days[3]))
	assert.Empty(t, slots(w.Days[5]))
	assert.Equal(t, "Estudos", w.Days[0].Blocks[2].Area)
}

func TestBuildWeekSplitsAtMidnight(t *testing.T) {
	_, _, _, _, sleep := fixtures(t)

	w := BuildWeek(monday, []FixedTask{sleep}, nil)

	// The Sunday before ends on this Monday; this Sunday starts here and ends next week.
	require.Equal(t, []slot{{"Dormir", "00:00", "07:00"}}, slots(w.Days[0]))
	assert.True(t, w.Days[0].Blocks[0].ContinuesFromPreviousDay)
	require.Equal(t, []slot{{"Dormir", "23:00", "24:00"}}, slots(w.Days[6]))
	assert.True(t, w.Days[6].Blocks[0].ContinuesNextDay)
}

func TestBuildWeekEndingAtMidnightHasNoSecondPart(t *testing.T) {
	late := FixedTask{ID: uuid.New(), Title: "Série", Area: "Lazer", Date: day(4), Start: at(22), End: at(0)}

	w := BuildWeek(monday, []FixedTask{late}, nil)

	assert.Equal(t, []slot{{"Série", "22:00", "24:00"}}, slots(w.Days[4]))
	assert.Empty(t, slots(w.Days[5]))
}

func TestBuildWeekAppliesAdjustments(t *testing.T) {
	work, class, _, _, _ := fixtures(t)
	adjustments := []Adjustment{
		{TaskID: class.ID, Date: day(0), Skipped: true},
		{TaskID: work.ID, Date: day(4), Start: at(9), End: at(16)},
		{TaskID: work.ID, Date: day(5), Start: at(9), End: at(12)}, // Saturday is not an occurrence
	}

	w := BuildWeek(monday, []FixedTask{work, class}, adjustments)

	assert.Equal(t, []slot{{"Trabalho", "09:00", "18:00"}}, slots(w.Days[0]), "class skipped this Monday")
	assert.Equal(t, []slot{{"Trabalho", "09:00", "16:00"}}, slots(w.Days[4]))
	assert.True(t, w.Days[4].Blocks[0].Adjusted)
	assert.False(t, w.Days[3].Blocks[0].Adjusted)
	assert.Empty(t, slots(w.Days[5]), "adjustments of non-occurrences are ignored")

	nextWeek := BuildWeek(day(7), []FixedTask{work, class}, nil)
	assert.Len(t, nextWeek.Days[0].Blocks, 2, "the pattern is unchanged")
}

func TestNewAdjustment(t *testing.T) {
	work, _, _, doctor, _ := fixtures(t)

	skip, err := NewAdjustment(userID, work, day(1), Change{Skip: true})
	require.NoError(t, err)
	moved, err := NewAdjustment(userID, doctor, day(3), Change{Start: at(17), End: at(18)})
	require.NoError(t, err)
	overnight, err := NewAdjustment(userID, work, day(2), Change{Start: at(22), End: at(6)})
	require.NoError(t, err)

	assert.Equal(t, Adjustment{UserID: userID, TaskID: work.ID, Date: day(1), Skipped: true}, skip)
	assert.Equal(t, Adjustment{UserID: userID, TaskID: doctor.ID, Date: day(3), Start: at(17), End: at(18)}, moved)
	assert.Equal(t, at(6), overnight.End)
}

func TestNewAdjustmentRejects(t *testing.T) {
	work, _, _, doctor, _ := fixtures(t)
	cases := map[string]struct {
		task   FixedTask
		date   calendar.Date
		change Change
		want   error
	}{
		"weekday not set":    {work, day(5), Change{Skip: true}, ErrNotAnOccurrence},
		"other date":         {doctor, day(4), Change{Skip: true}, ErrNotAnOccurrence},
		"same start and end": {work, day(0), Change{Start: at(9), End: at(9)}, ErrInvalidTimeRange},
		"time out of range":  {work, day(0), Change{Start: at(9), End: EndOfDay}, calendar.ErrInvalidTime},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewAdjustment(userID, tc.task, tc.date, tc.change)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}
