package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

func TestSaveSessionSwitchesBetweenTimeAndPeriod(t *testing.T) {
	f := newSessionFixture(t)
	task := f.recurring(userA)
	inPeriod := domain.Session{UserID: userA, TaskID: task, Date: monday, Period: calendar.Evening}

	saved, err := f.sessions.Save(t.Context(), session(userA, task, monday, 7*60, 8*60))
	require.NoError(t, err)
	_, err = f.sessions.Save(t.Context(), inPeriod)
	require.NoError(t, err)
	got := f.listSessions(userA)
	require.Len(t, got, 1)
	assert.Equal(t, calendar.Evening, got[0].Period)
	assert.Zero(t, got[0].Start)
	assert.Zero(t, got[0].End)
	assert.Equal(t, saved.CreatedAt, got[0].CreatedAt)

	_, err = f.sessions.Save(t.Context(), session(userA, task, monday, 19*60, 20*60))
	require.NoError(t, err)
	got = f.listSessions(userA)
	require.Len(t, got, 1)
	assert.True(t, got[0].Timed())
	assert.Equal(t, calendar.TimeOfDay(19*60), got[0].Start)
}

func TestDatabaseRequiresATimeOrAPeriod(t *testing.T) {
	f := newSessionFixture(t)
	task := f.recurring(userA)
	cases := map[string]sessionModel{
		"neither":             {},
		"start without end":   {StartMinute: new(7 * 60)},
		"time and period":     {StartMinute: new(7 * 60), EndMinute: new(8 * 60), Period: new("evening")},
		"period and only end": {EndMinute: new(8 * 60), Period: new("evening")},
		"unknown period":      {Period: new("dawn")},
	}

	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			m.TaskID, m.Date, m.UserID = task, monday.Time(), userA

			// A nested transaction rolls back to a savepoint, keeping the test's open.
			err := f.tx.Transaction(func(tx *gorm.DB) error { return tx.Create(&m).Error })

			assert.Error(t, err)
		})
	}
}
