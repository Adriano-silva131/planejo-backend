package app

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

var (
	userID = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")
	monday = calendar.NewDate(2026, time.October, 26)
	class  = domain.FixedTask{
		ID: uuid.New(), Title: "Faculdade", Area: "Estudos",
		Weekdays: calendar.Weekdays(1 << time.Monday), Start: 19 * 60, End: 22 * 60,
	}
)

// fakeTasks knows class; with notFixed set, class is no longer fixed.
type fakeTasks struct {
	notFixed bool
}

func (fakeTasks) Fixed(context.Context, uuid.UUID, calendar.Date, calendar.Date) ([]domain.FixedTask, error) {
	return []domain.FixedTask{class}, nil
}

func (f fakeTasks) Get(_ context.Context, _, id uuid.UUID) (domain.FixedTask, error) {
	switch {
	case id != class.ID:
		return domain.FixedTask{}, domain.ErrTaskNotFound
	case f.notFixed:
		return domain.FixedTask{}, domain.ErrNotFixedTask
	}
	return class, nil
}

type fakeRepo struct {
	saved    []domain.Adjustment
	from, to calendar.Date
}

func (r *fakeRepo) Save(_ context.Context, a domain.Adjustment) (domain.Adjustment, error) {
	r.saved = append(r.saved, a)
	return a, nil
}

func (r *fakeRepo) Delete(_ context.Context, _, _ uuid.UUID, date calendar.Date) error {
	r.saved = slices.DeleteFunc(r.saved, func(a domain.Adjustment) bool { return a.Date.Equal(date) })
	return nil
}

func (r *fakeRepo) ForTask(context.Context, uuid.UUID, uuid.UUID) ([]domain.Adjustment, error) {
	return r.saved, nil
}

func (r *fakeRepo) List(_ context.Context, _ uuid.UUID, from, to calendar.Date) ([]domain.Adjustment, error) {
	r.from, r.to = from, to
	return r.saved, nil
}

func TestWeekLoadsAdjustmentsFromTheSundayBefore(t *testing.T) {
	repo := &fakeRepo{}

	w, err := NewRoutine(fakeTasks{}, repo).Week(t.Context(), userID, monday.AddDays(3))

	require.NoError(t, err)
	assert.Equal(t, monday.AddDays(-1), repo.from)
	assert.Equal(t, monday.AddDays(6), repo.to)
	assert.Len(t, w.Days[0].Blocks, 1)
}

func TestAdjustSkipsAnOccurrenceOnlyThatWeek(t *testing.T) {
	repo := &fakeRepo{}
	r := NewRoutine(fakeTasks{}, repo)

	_, err := r.Adjust(t.Context(), userID, class.ID, monday, domain.Change{Skip: true})
	require.NoError(t, err)
	thisWeek, err := r.Week(t.Context(), userID, monday)
	require.NoError(t, err)
	nextWeek, err := r.Week(t.Context(), userID, monday.AddDays(7))
	require.NoError(t, err)

	assert.Empty(t, thisWeek.Days[0].Blocks)
	assert.Len(t, nextWeek.Days[0].Blocks, 1)
}

func TestAdjustValidatesBeforeSaving(t *testing.T) {
	repo := &fakeRepo{}
	r := NewRoutine(fakeTasks{}, repo)

	_, notOccurrence := r.Adjust(t.Context(), userID, class.ID, monday.AddDays(1), domain.Change{Skip: true})
	_, unknown := r.Adjust(t.Context(), userID, uuid.New(), monday, domain.Change{Skip: true})

	assert.ErrorIs(t, notOccurrence, domain.ErrNotAnOccurrence)
	assert.ErrorIs(t, unknown, domain.ErrTaskNotFound)
	assert.Empty(t, repo.saved)
}

func TestScheduleChangedDropsAdjustmentsOfDatesThatNoLongerOccur(t *testing.T) {
	// class is on Mondays; the Tuesday adjustment is left from an older schedule.
	repo := &fakeRepo{saved: []domain.Adjustment{
		{TaskID: class.ID, Date: monday, Skipped: true},
		{TaskID: class.ID, Date: monday.AddDays(1), Skipped: true},
	}}

	require.NoError(t, NewRoutine(fakeTasks{}, repo).ScheduleChanged(t.Context(), userID, class.ID))

	require.Len(t, repo.saved, 1)
	assert.Equal(t, monday, repo.saved[0].Date)
}

func TestScheduleChangedDropsEverythingWhenNoLongerFixed(t *testing.T) {
	repo := &fakeRepo{saved: []domain.Adjustment{{TaskID: class.ID, Date: monday, Skipped: true}}}

	require.NoError(t, NewRoutine(fakeTasks{notFixed: true}, repo).ScheduleChanged(t.Context(), userID, class.ID))

	assert.Empty(t, repo.saved)
}
