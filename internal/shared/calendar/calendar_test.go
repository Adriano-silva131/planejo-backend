package calendar

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

func TestWeekdayNames(t *testing.T) {
	w, err := ParseWeekdayNames([]string{"friday", "monday"})

	require.NoError(t, err)
	assert.Equal(t, []string{"monday", "friday"}, w.Names())
	weekend, err := ParseWeekdayNames([]string{"sunday", "saturday"})
	require.NoError(t, err)
	assert.Equal(t, []string{"saturday", "sunday"}, weekend.Names(), "Monday first, Sunday last")
	assert.Equal(t, 2, weekend.Count())
	assert.Equal(t, NewDate(2026, time.October, 29), DateOf(time.Date(2026, time.October, 29, 23, 0, 0, 0, time.UTC)),
		"comparable with ==")
	assert.Equal(t, "sunday", WeekdayName(time.Sunday))
	assert.Nil(t, Weekdays(0).Names())
	_, err = ParseWeekdayNames([]string{"mon"})
	assert.ErrorIs(t, err, ErrInvalidWeekday)
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

func TestZeroDateIsEmpty(t *testing.T) {
	d, err := ParseOptionalDate("")

	require.NoError(t, err)
	assert.True(t, d.IsZero())
	assert.Empty(t, Date{}.String())
	_, err = ParseOptionalDate("30/10/2026")
	assert.ErrorIs(t, err, ErrInvalidDate)
}

func TestDateRejectsInvalidValues(t *testing.T) {
	for _, s := range []string{"2026-02-30", "30/10/2026", ""} {
		_, err := ParseDate(s)

		assert.ErrorIs(t, err, ErrInvalidDate, s)
	}
}

func TestDateArithmetic(t *testing.T) {
	thursday := NewDate(2026, time.October, 29)

	assert.Equal(t, time.Thursday, thursday.Weekday())
	assert.Equal(t, NewDate(2026, time.November, 2), thursday.AddDays(4))
	assert.True(t, thursday.Before(thursday.AddDays(1)))
	assert.True(t, thursday.After(thursday.AddDays(-1)))
	assert.False(t, thursday.After(thursday))
	parsed, err := ParseDate("2026-10-29")
	require.NoError(t, err)
	assert.True(t, thursday.Equal(parsed))
	assert.False(t, thursday.Equal(thursday.AddDays(1)))
}

func TestStartOfWeekIsMonday(t *testing.T) {
	monday := NewDate(2026, time.October, 26)

	for offset := range 7 {
		assert.Equal(t, monday, monday.AddDays(offset).StartOfWeek(), monday.AddDays(offset).Weekday())
	}
	assert.Equal(t, NewDate(2026, time.November, 2), NewDate(2026, time.November, 2).StartOfWeek())
}

func TestDateOfUsesTheTimeLocation(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	instant := time.Date(2026, time.October, 27, 1, 0, 0, 0, time.UTC)

	assert.Equal(t, NewDate(2026, time.October, 26), DateOf(instant.In(saoPaulo)))
	assert.Equal(t, NewDate(2026, time.October, 27), DateOf(instant))
}

func TestLocationInContext(t *testing.T) {
	saoPaulo := time.FixedZone("BRT", -3*60*60)

	assert.Equal(t, time.UTC, LocationFrom(t.Context()))
	assert.Equal(t, saoPaulo, LocationFrom(WithLocation(t.Context(), saoPaulo)))
}
