package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeekdays(t *testing.T) {
	w, err := NewWeekdays(time.Friday, time.Monday, time.Monday)

	require.NoError(t, err)
	assert.Equal(t, []time.Weekday{time.Monday, time.Friday}, w.Days())
	assert.True(t, w.Has(time.Friday))
	assert.False(t, w.Has(time.Sunday))
	assert.False(t, w.IsEmpty())
}

func TestWeekdaysRejectsUnknownDay(t *testing.T) {
	_, err := NewWeekdays(time.Weekday(7))

	assert.ErrorIs(t, err, ErrInvalidWeekday)
}

func TestTimeOfDay(t *testing.T) {
	tod, err := ParseTimeOfDay("07:30")

	require.NoError(t, err)
	assert.Equal(t, TimeOfDay(450), tod)
	assert.Equal(t, "07:30", tod.String())
}

func TestTimeOfDayRejectsInvalidValues(t *testing.T) {
	for _, s := range []string{"24:00", "7:5", "07:60", "", "seven"} {
		_, err := ParseTimeOfDay(s)

		assert.ErrorIs(t, err, ErrInvalidTime, s)
	}
	_, err := NewTimeOfDay(-1, 0)
	assert.ErrorIs(t, err, ErrInvalidTime)
}

func TestDate(t *testing.T) {
	d, err := ParseDate("2026-10-30")

	require.NoError(t, err)
	assert.Equal(t, NewDate(2026, time.October, 30), d)
	assert.Equal(t, "2026-10-30", d.String())
	assert.Equal(t, time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC), d.Time())
	assert.True(t, Date{}.IsZero())
}

func TestDateRejectsInvalidValues(t *testing.T) {
	for _, s := range []string{"2026-02-30", "30/10/2026", ""} {
		_, err := ParseDate(s)

		assert.ErrorIs(t, err, ErrInvalidDate, s)
	}
}
