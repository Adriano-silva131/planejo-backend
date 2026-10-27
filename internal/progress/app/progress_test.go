package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

var (
	userID      = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")
	gym         = domain.Task{ID: uuid.New(), Title: "Academia", Kind: domain.RecurringTask, TimesPerWeek: 3}
	errNotFound = domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")
)

type fakeTasks map[uuid.UUID]domain.Task

func (f fakeTasks) Get(_ context.Context, _, id uuid.UUID) (domain.Task, error) {
	t, ok := f[id]
	if !ok {
		return domain.Task{}, errNotFound
	}
	return t, nil
}

func (f fakeTasks) List(context.Context, uuid.UUID) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f {
		out = append(out, t)
	}
	return out, nil
}

type fakeRepo struct {
	saved  []domain.CheckIn
	filter domain.Filter
}

func (r *fakeRepo) Save(_ context.Context, c domain.CheckIn) (domain.CheckIn, error) {
	r.saved = append(r.saved, c)
	return c, nil
}

func (r *fakeRepo) Delete(context.Context, uuid.UUID, uuid.UUID, calendar.Date) error {
	return nil
}

func (r *fakeRepo) List(_ context.Context, _ uuid.UUID, f domain.Filter) ([]domain.CheckIn, error) {
	r.filter = f
	return r.saved, nil
}

func newProgress(repo *fakeRepo, now time.Time) *Progress {
	p := NewProgress(fakeTasks{gym.ID: gym}, repo)
	p.now = func() time.Time { return now }
	return p
}

func TestCheckInSavesValidEntry(t *testing.T) {
	repo := &fakeRepo{}
	day := calendar.NewDate(2026, time.October, 27)

	_, err := newProgress(repo, time.Date(2026, time.October, 27, 12, 0, 0, 0, time.UTC)).
		CheckIn(t.Context(), userID, gym.ID, domain.Entry{Date: day})

	require.NoError(t, err)
	assert.Equal(t, []domain.CheckIn{{UserID: userID, TaskID: gym.ID, Date: day}}, repo.saved)
}

func TestCheckInTodayIsTheLatestDateAnywhere(t *testing.T) {
	// 11:00 UTC on the 27th is already the 28th in UTC+14, but not the 29th.
	now := time.Date(2026, time.October, 27, 11, 0, 0, 0, time.UTC)
	p := newProgress(&fakeRepo{}, now)

	_, tomorrow := p.CheckIn(t.Context(), userID, gym.ID, domain.Entry{Date: calendar.NewDate(2026, time.October, 28)})
	_, later := p.CheckIn(t.Context(), userID, gym.ID, domain.Entry{Date: calendar.NewDate(2026, time.October, 29)})

	assert.NoError(t, tomorrow)
	assert.ErrorIs(t, later, domain.ErrFutureCheckIn)
}

func TestCheckInOfUnknownTask(t *testing.T) {
	repo := &fakeRepo{}

	_, err := newProgress(repo, time.Now()).CheckIn(t.Context(), userID, uuid.New(), domain.Entry{})

	assert.ErrorIs(t, err, errNotFound)
	assert.Empty(t, repo.saved)
}

func TestHistoryChecksTheTaskFirst(t *testing.T) {
	_, err := newProgress(&fakeRepo{}, time.Now()).History(t.Context(), userID, domain.Filter{TaskID: uuid.New()})

	assert.ErrorIs(t, err, errNotFound)
}

func TestWeekLoadsCheckInsUntilTheEndOfTheWeek(t *testing.T) {
	repo := &fakeRepo{}
	wednesday := calendar.NewDate(2026, time.October, 28)

	s, err := newProgress(repo, time.Now()).Week(t.Context(), userID, wednesday)

	require.NoError(t, err)
	assert.Equal(t, domain.Filter{To: calendar.NewDate(2026, time.November, 1)}, repo.filter)
	assert.Equal(t, calendar.NewDate(2026, time.October, 26), s.Start)
	require.Len(t, s.Recurring, 1)
}
