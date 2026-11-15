package calendar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPeriodOf(t *testing.T) {
	cases := map[string]Period{
		"00:00": Morning, "11:59": Morning, "12:00": Afternoon, "17:59": Afternoon, "18:00": Evening, "23:59": Evening,
	}
	for at, want := range cases {
		tod, err := ParseTimeOfDay(at)

		assert.NoError(t, err)
		assert.Equal(t, want, PeriodOf(tod), at)
	}
}

func TestPeriodValid(t *testing.T) {
	assert.True(t, Evening.Valid())
	assert.False(t, Period("").Valid())
	assert.False(t, Period("dawn").Valid())
}
