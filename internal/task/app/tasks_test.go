package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

var userID = uuid.MustParse("6f1b5d1e-6a3c-4a8e-9a5b-2f1d7c3e8b90")

type fakeRepo struct {
	tasks        map[uuid.UUID]domain.Task
	offset       int
	limit        int
	updateCalled bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tasks: map[uuid.UUID]domain.Task{}}
}

func (r *fakeRepo) Create(_ context.Context, t domain.Task) (domain.Task, error) {
	r.tasks[t.ID] = t
	return t, nil
}

func (r *fakeRepo) Get(_ context.Context, userID, id uuid.UUID) (domain.Task, error) {
	t, ok := r.tasks[id]
	if !ok || t.UserID != userID {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	return t, nil
}

func (r *fakeRepo) List(_ context.Context, _ uuid.UUID, offset, limit int) ([]domain.Task, int, error) {
	r.offset, r.limit = offset, limit
	return nil, 0, nil
}

func (r *fakeRepo) All(_ context.Context, userID uuid.UUID) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range r.tasks {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *fakeRepo) Areas(context.Context, uuid.UUID) ([]domain.AreaUsage, error) {
	return nil, nil
}

func (r *fakeRepo) Update(_ context.Context, t domain.Task) (domain.Task, error) {
	r.updateCalled = true
	r.tasks[t.ID] = t
	return t, nil
}

func (r *fakeRepo) Delete(_ context.Context, _, id uuid.UUID) error {
	delete(r.tasks, id)
	return nil
}

func gym(t *testing.T) domain.Details {
	t.Helper()
	days, err := calendar.NewWeekdays(time.Monday, time.Wednesday)
	require.NoError(t, err)
	return domain.Details{
		Title:    "Academia",
		Area:     "Saúde",
		Priority: domain.High,
		Schedule: domain.RecurringSchedule{Weekdays: days, Duration: time.Hour},
	}
}

func TestCreateStoresValidTask(t *testing.T) {
	repo := newFakeRepo()

	task, err := NewTasks(repo).Create(t.Context(), userID, gym(t))

	require.NoError(t, err)
	assert.Equal(t, userID, task.UserID)
	assert.Contains(t, repo.tasks, task.ID)
}

func TestCreateRejectsInvalidTask(t *testing.T) {
	repo := newFakeRepo()

	_, err := NewTasks(repo).Create(t.Context(), userID, domain.Details{Title: "Academia", Area: "Saúde"})

	assert.ErrorIs(t, err, domain.ErrScheduleRequired)
	assert.Empty(t, repo.tasks)
}

func TestListTurnsPageIntoOffset(t *testing.T) {
	repo := newFakeRepo()

	_, _, err := NewTasks(repo).List(t.Context(), userID, 3, 20)

	require.NoError(t, err)
	assert.Equal(t, 40, repo.offset)
	assert.Equal(t, 20, repo.limit)
}

func TestUpdateValidatesBeforeSaving(t *testing.T) {
	repo := newFakeRepo()
	tasks := NewTasks(repo)
	task, err := tasks.Create(t.Context(), userID, gym(t))
	require.NoError(t, err)

	_, err = tasks.Update(t.Context(), userID, task.ID, domain.Details{Title: ""})

	assert.ErrorIs(t, err, domain.ErrTitleRequired)
	assert.False(t, repo.updateCalled)
}

func TestUpdateOfMissingTask(t *testing.T) {
	_, err := NewTasks(newFakeRepo()).Update(t.Context(), userID, uuid.New(), gym(t))

	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
}
