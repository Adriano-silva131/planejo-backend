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
	gym = domain.PlannableTask{ID: uuid.New(), Title: "Academia", Area: "Saúde", Kind: domain.RecurringKind, Duration: time.Hour}
)

// fakeTasks stands in for the task module; newRoutine gives it class, which
// is fixed, and gym, which is recurring.
type fakeTasks struct {
	fixed     []domain.FixedTask
	plannable []domain.PlannableTask
}

func (f *fakeTasks) List(context.Context, uuid.UUID) ([]domain.FixedTask, []domain.PlannableTask, error) {
	return f.fixed, f.plannable, nil
}

func (f *fakeTasks) GetFixed(_ context.Context, _, id uuid.UUID) (domain.FixedTask, error) {
	if i := slices.IndexFunc(f.fixed, func(t domain.FixedTask) bool { return t.ID == id }); i >= 0 {
		return f.fixed[i], nil
	}
	if slices.ContainsFunc(f.plannable, func(t domain.PlannableTask) bool { return t.ID == id }) {
		return domain.FixedTask{}, domain.ErrNoOccurrences
	}
	return domain.FixedTask{}, domain.ErrTaskNotFound
}

func (f *fakeTasks) GetPlannable(_ context.Context, _, id uuid.UUID) (domain.PlannableTask, error) {
	if i := slices.IndexFunc(f.plannable, func(t domain.PlannableTask) bool { return t.ID == id }); i >= 0 {
		return f.plannable[i], nil
	}
	if slices.ContainsFunc(f.fixed, func(t domain.FixedTask) bool { return t.ID == id }) {
		return domain.PlannableTask{}, domain.ErrFixedTaskSession
	}
	return domain.PlannableTask{}, domain.ErrTaskNotFound
}

// fakeTx runs fn as is; ctx tells fn it runs in a transaction.
type fakeTx struct{}

type inTxKey struct{}

func (fakeTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(context.WithValue(ctx, inTxKey{}, true))
}

type fakeRepo struct {
	saved    []domain.Adjustment
	from, to calendar.Date
	// locks counts the locks taken inside a transaction.
	locks int
}

func (r *fakeRepo) Lock(ctx context.Context, _ uuid.UUID) error {
	if in, _ := ctx.Value(inTxKey{}).(bool); in {
		r.locks++
	}
	return nil
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

type fakeSessions struct {
	saved    []domain.Session
	from, to calendar.Date
}

func (s *fakeSessions) Save(_ context.Context, session domain.Session) (domain.Session, error) {
	s.saved = append(s.saved, session)
	return session, nil
}

func (s *fakeSessions) Delete(_ context.Context, _, _ uuid.UUID, date calendar.Date) error {
	s.saved = slices.DeleteFunc(s.saved, func(session domain.Session) bool { return session.Date.Equal(date) })
	return nil
}

func (s *fakeSessions) List(_ context.Context, _ uuid.UUID, from, to calendar.Date) ([]domain.Session, error) {
	s.from, s.to = from, to
	return s.saved, nil
}

func (s *fakeSessions) DeleteForTask(context.Context, uuid.UUID, uuid.UUID) error {
	s.saved = nil
	return nil
}

func newRoutine(repo *fakeRepo) (*Routine, *fakeSessions) {
	r, sessions, _ := newRoutineWith(repo)
	return r, sessions
}

func newRoutineWith(repo *fakeRepo) (*Routine, *fakeSessions, *fakeTasks) {
	sessions := &fakeSessions{}
	tasks := &fakeTasks{fixed: []domain.FixedTask{class}, plannable: []domain.PlannableTask{gym}}
	return NewRoutine(tasks, repo, sessions, fakeTx{}), sessions, tasks
}

func TestWeekLoadsAdjustmentsFromTheSundayBeforeAndSessionsOfTheWeek(t *testing.T) {
	repo := &fakeRepo{}
	r, sessions := newRoutine(repo)

	w, err := r.Week(t.Context(), userID, monday.AddDays(3))

	require.NoError(t, err)
	assert.Equal(t, monday.AddDays(-1), repo.from)
	assert.Equal(t, monday.AddDays(6), repo.to)
	assert.Equal(t, monday, sessions.from)
	assert.Equal(t, monday.AddDays(6), sessions.to)
	assert.Len(t, w.Days[0].Blocks, 1)
}

func TestPlaceSessionShowsInTheWeek(t *testing.T) {
	r, _ := newRoutine(&fakeRepo{})
	saturday := monday.AddDays(5)

	s, err := r.PlaceSession(t.Context(), userID, gym.ID, saturday, domain.Placement{Start: calendar.SomeTime(10 * 60)})
	require.NoError(t, err)
	w, err := r.Week(t.Context(), userID, monday)
	require.NoError(t, err)

	assert.Equal(t, calendar.TimeOfDay(11*60), s.End)
	require.Len(t, w.Days[5].Blocks, 1)
	assert.Equal(t, "Academia", w.Days[5].Blocks[0].Title)
}

func TestPlaceSessionRejectsFixedAndUnknownTasks(t *testing.T) {
	r, sessions := newRoutine(&fakeRepo{})

	_, fixed := r.PlaceSession(t.Context(), userID, class.ID, monday, domain.Placement{Start: calendar.SomeTime(8 * 60)})
	_, unknown := r.PlaceSession(t.Context(), userID, uuid.New(), monday, domain.Placement{Start: calendar.SomeTime(8 * 60)})
	_, occurrence := r.Adjust(t.Context(), userID, gym.ID, monday, domain.Change{Skip: true})

	assert.ErrorIs(t, fixed, domain.ErrFixedTaskSession)
	assert.ErrorIs(t, unknown, domain.ErrTaskNotFound)
	assert.ErrorIs(t, occurrence, domain.ErrNoOccurrences)
	assert.Empty(t, sessions.saved)
}

func TestAdjustSkipsAnOccurrenceOnlyThatWeek(t *testing.T) {
	repo := &fakeRepo{}
	r, _ := newRoutine(repo)

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
	r, _ := newRoutine(repo)

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

	r, _ := newRoutine(repo)

	require.NoError(t, r.ScheduleChanged(t.Context(), userID, class.ID, &class))

	require.Len(t, repo.saved, 1)
	assert.Equal(t, monday, repo.saved[0].Date)
}

func TestScheduleChangedDropsAdjustmentsOfATaskNoLongerFixed(t *testing.T) {
	repo := &fakeRepo{saved: []domain.Adjustment{{TaskID: class.ID, Date: monday, Skipped: true}}}
	r, sessions := newRoutine(repo)
	sessions.saved = []domain.Session{{TaskID: class.ID, Date: monday, Start: 8 * 60, End: 9 * 60}}

	require.NoError(t, r.ScheduleChanged(t.Context(), userID, class.ID, nil))

	assert.Empty(t, repo.saved)
	assert.Len(t, sessions.saved, 1, "sessions stay while the task is not fixed")
}

func TestScheduleChangedDropsSessionsOfATaskThatBecameFixed(t *testing.T) {
	r, sessions := newRoutine(&fakeRepo{})
	sessions.saved = []domain.Session{{TaskID: class.ID, Date: monday, Start: 8 * 60, End: 9 * 60}}

	require.NoError(t, r.ScheduleChanged(t.Context(), userID, class.ID, &class))

	assert.Empty(t, sessions.saved)
}
