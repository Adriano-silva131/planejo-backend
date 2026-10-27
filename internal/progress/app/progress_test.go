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
	saved     []domain.CheckIn
	filter    domain.Filter
	schedules map[uuid.UUID][]domain.PastSchedule
}

func (r *fakeRepo) SaveSchedule(_ context.Context, _, taskID uuid.UUID, s domain.PastSchedule) error {
	if r.schedules == nil {
		r.schedules = map[uuid.UUID][]domain.PastSchedule{}
	}
	r.schedules[taskID] = append(r.schedules[taskID], s)
	return nil
}

func (r *fakeRepo) Schedules(context.Context, uuid.UUID) (map[uuid.UUID][]domain.PastSchedule, error) {
	return r.schedules, nil
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
		CheckIn(t.Context(), userID, gym.ID, domain.Entry{Date: day}, time.UTC)

	require.NoError(t, err)
	assert.Equal(t, []domain.CheckIn{{UserID: userID, TaskID: gym.ID, Date: day}}, repo.saved)
}

func TestCheckInTodayIsInTheUsersTimeZone(t *testing.T) {
	// 01:00 UTC on the 28th: still the 27th in São Paulo, already the 28th in Tokyo.
	now := time.Date(2026, time.October, 28, 1, 0, 0, 0, time.UTC)
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	tokyo := time.FixedZone("JST", 9*60*60)
	p := newProgress(&fakeRepo{}, now)
	the28th := domain.Entry{Date: calendar.NewDate(2026, time.October, 28)}

	_, inSaoPaulo := p.CheckIn(t.Context(), userID, gym.ID, the28th, saoPaulo)
	_, inTokyo := p.CheckIn(t.Context(), userID, gym.ID, the28th, tokyo)

	assert.ErrorIs(t, inSaoPaulo, domain.ErrFutureCheckIn)
	assert.NoError(t, inTokyo)
}

func TestCheckInOfUnknownTask(t *testing.T) {
	repo := &fakeRepo{}

	_, err := newProgress(repo, time.Now()).CheckIn(t.Context(), userID, uuid.New(), domain.Entry{}, time.UTC)

	assert.ErrorIs(t, err, errNotFound)
	assert.Empty(t, repo.saved)
}

func TestHistoryChecksTheTaskFirst(t *testing.T) {
	_, err := newProgress(&fakeRepo{}, time.Now()).History(t.Context(), userID, domain.Filter{TaskID: uuid.New()})

	assert.ErrorIs(t, err, errNotFound)
}

func TestHabitsUseTheUsersTimeZone(t *testing.T) {
	// Created Sunday 2026-10-25 at 22:00 in São Paulo, already Monday in UTC.
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	reading := domain.Task{
		ID: uuid.New(), Title: "Leitura", Kind: domain.RecurringTask,
		Weekdays: calendar.Weekdays(1 << time.Sunday), TimesPerWeek: 1,
		CreatedAt: time.Date(2026, time.October, 26, 1, 0, 0, 0, time.UTC),
		CreatedOn: calendar.NewDate(2026, time.October, 26),
	}
	repo := &fakeRepo{saved: []domain.CheckIn{{TaskID: reading.ID, Date: calendar.NewDate(2026, time.October, 25)}}}
	p := NewProgress(fakeTasks{reading.ID: reading}, repo)
	// Monday 2026-10-26 at 01:00 in São Paulo: the week of the 19th is over.
	p.now = func() time.Time { return time.Date(2026, time.October, 26, 4, 0, 0, 0, time.UTC) }

	habits, err := p.Habits(t.Context(), userID, calendar.NewDate(2026, time.October, 25), 4, saoPaulo)

	require.NoError(t, err)
	require.Len(t, habits, 1, "created on Sunday in the user's time zone")
	require.Len(t, habits[0].Weeks, 1)
	assert.True(t, habits[0].Weeks[0].Met)
	assert.Equal(t, 1, habits[0].Streak)
	assert.Equal(t, domain.Filter{
		TaskIDs: []uuid.UUID{reading.ID},
		From:    calendar.NewDate(2026, time.October, 19),
		To:      calendar.NewDate(2026, time.October, 25),
	}, repo.filter, "only the habits' check-ins, from the oldest habit's week")
}

func TestScheduleChangedKeepsTheOldWeekdays(t *testing.T) {
	mon := calendar.Weekdays(1 << time.Monday)
	monWed := mon | 1<<time.Wednesday
	// Monday 2026-11-02 at 01:00 UTC: still Sunday the 1st in São Paulo.
	now := time.Date(2026, time.November, 2, 1, 0, 0, 0, time.UTC)
	inSaoPaulo := calendar.WithLocation(t.Context(), time.FixedZone("BRT", -3*60*60))
	cases := map[string]struct {
		ctx           context.Context
		before, after calendar.Weekdays
		want          []domain.PastSchedule
	}{
		"new weekdays, user's zone": {inSaoPaulo, mon, monWed, []domain.PastSchedule{
			{ChangedOn: calendar.NewDate(2026, time.November, 1), Weekdays: mon},
		}},
		"no zone, UTC": {t.Context(), mon, monWed, []domain.PastSchedule{
			{ChangedOn: calendar.NewDate(2026, time.November, 2), Weekdays: mon},
		}},
		"became recurring": {t.Context(), 0, mon, []domain.PastSchedule{{ChangedOn: calendar.NewDate(2026, time.November, 2)}}},
		"same weekdays":    {t.Context(), mon, mon, nil},
		"never recurring":  {t.Context(), 0, 0, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}

			require.NoError(t, newProgress(repo, now).ScheduleChanged(tc.ctx, userID, gym.ID, tc.before, tc.after))

			assert.Equal(t, tc.want, repo.schedules[gym.ID])
		})
	}
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
