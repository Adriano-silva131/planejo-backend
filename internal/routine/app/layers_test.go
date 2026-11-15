package app

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

func assertOverlap(t *testing.T, err error) {
	t.Helper()
	var de *domainerr.Error
	require.True(t, errors.As(err, &de), "want a schedule_overlap error, got %v", err)
	assert.Equal(t, domain.ScheduleOverlapCode, de.Code)
}

// dinner is on Monday at the start of class, so the evening has two layers.
var dinner = domain.FixedTask{
	ID: uuid.New(), Title: "Jantar", Weekdays: calendar.Weekdays(1 << time.Monday), Start: 19 * 60, End: 20 * 60,
}

func TestPlaceSessionRefusesAThirdLayer(t *testing.T) {
	repo := &fakeRepo{}
	r, _, tasks := newRoutineWith(repo)
	tasks.fixed = append(tasks.fixed, dinner)

	_, err := r.PlaceSession(t.Context(), userID, gym.ID, monday, domain.Placement{Start: calendar.SomeTime(19*60 + 30)})

	assertOverlap(t, err)
	assert.Equal(t, 1, repo.locks, "checked under the lock, in a transaction")
}

func TestPlaceSessionBesideTwoLayers(t *testing.T) {
	r, _, tasks := newRoutineWith(&fakeRepo{})
	tasks.fixed = append(tasks.fixed, dinner)

	_, err := r.PlaceSession(t.Context(), userID, gym.ID, monday, domain.Placement{Start: calendar.SomeTime(20 * 60)})

	assert.NoError(t, err)
}

func TestAdjustChecksTheDayAndTheNext(t *testing.T) {
	repo := &fakeRepo{}
	r, _, tasks := newRoutineWith(repo)
	tasks.fixed = append(tasks.fixed, dinner)

	_, err := r.Adjust(t.Context(), userID, class.ID, monday, domain.Change{Start: 23 * 60, End: 1 * 60})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.locks)
	assert.Equal(t, monday.AddDays(-1), repo.from)
	assert.Equal(t, monday.AddDays(6), repo.to)
}

func TestAdjustToTheTimeOfTwoLayers(t *testing.T) {
	r, sessions, tasks := newRoutineWith(&fakeRepo{})
	tasks.fixed = append(tasks.fixed, dinner)
	sessions.saved = []domain.Session{{TaskID: gym.ID, Date: monday, Start: 17 * 60, End: 18 * 60}}

	_, err := r.Adjust(t.Context(), userID, dinner.ID, monday, domain.Change{Start: 17 * 60, End: 18 * 60})
	assert.NoError(t, err, "dinner and gym are two layers")
	_, err = r.Adjust(t.Context(), userID, class.ID, monday, domain.Change{Start: 17 * 60, End: 19 * 60})

	assertOverlap(t, err)
}

func TestSkippingAnOccurrenceIsNeverChecked(t *testing.T) {
	repo := &fakeRepo{}
	r, _ := newRoutine(repo)

	_, err := r.Adjust(t.Context(), userID, class.ID, monday, domain.Change{Skip: true})

	require.NoError(t, err)
	assert.Zero(t, repo.locks)
}

func TestResetIsChecked(t *testing.T) {
	r, sessions, tasks := newRoutineWith(&fakeRepo{saved: []domain.Adjustment{{TaskID: class.ID, Date: monday, Skipped: true}}})
	tasks.fixed = append(tasks.fixed, dinner)
	sessions.saved = []domain.Session{{TaskID: gym.ID, Date: monday, Start: 19 * 60, End: 20 * 60}}

	err := r.Reset(t.Context(), userID, class.ID, monday)

	assertOverlap(t, err)
}

func TestCheckFixedWeeklyStartsInTheUsersCurrentWeek(t *testing.T) {
	repo := &fakeRepo{}
	r, sessions, _ := newRoutineWith(repo)
	// Monday 2026-11-02 at 01:00 UTC: still Sunday the 1st in São Paulo.
	r.now = func() time.Time { return time.Date(2026, time.November, 2, 1, 0, 0, 0, time.UTC) }
	ctx := calendar.WithLocation(t.Context(), time.FixedZone("BRT", -3*60*60))

	require.NoError(t, r.CheckFixed(ctx, userID, class))

	assert.Equal(t, monday, sessions.from)
	assert.True(t, sessions.to.IsZero(), "every later week")
	assert.Equal(t, monday.AddDays(-1), repo.from)
}

func TestCheckFixedOnADate(t *testing.T) {
	r, sessions, tasks := newRoutineWith(&fakeRepo{})
	appointment := domain.FixedTask{ID: uuid.New(), Title: "Consulta", Date: monday.AddDays(9), Start: 19 * 60, End: 20 * 60}
	tasks.fixed = append(tasks.fixed, dinner, appointment)

	require.NoError(t, r.CheckFixed(t.Context(), userID, appointment))
	assert.Equal(t, monday.AddDays(7), sessions.from)

	appointment.Date = monday
	tasks.fixed[len(tasks.fixed)-1] = appointment
	assertOverlap(t, r.CheckFixed(t.Context(), userID, appointment))
}
