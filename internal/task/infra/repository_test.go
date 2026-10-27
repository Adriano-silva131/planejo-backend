package infra

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

var (
	userA = uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90")
	userB = uuid.MustParse("0b9c3f0e-2222-4a8e-9a5b-2f1d7c3e8b90")
)

func TestMain(m *testing.M) {
	dbtest.Main(m)
}

func newRepo(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(dbtest.Tx(t))
}

func weekdays(t *testing.T, days ...time.Weekday) calendar.Weekdays {
	t.Helper()
	w, err := calendar.NewWeekdays(days...)
	require.NoError(t, err)
	return w
}

func details(t *testing.T) map[string]domain.Details {
	t.Helper()
	return map[string]domain.Details{
		"fixed weekly": {Title: "Trabalho", Area: "Trabalho", Schedule: domain.FixedSchedule{
			Weekdays: weekdays(t, time.Monday, time.Friday), Start: 9 * 60, End: 18 * 60,
		}},
		"fixed on date": {Title: "Consulta", Schedule: domain.FixedSchedule{
			Date: calendar.NewDate(2026, time.November, 5), Start: 15 * 60, End: 16 * 60,
		}},
		"recurring": {Title: "Academia", Area: "Saúde", Priority: domain.High, Schedule: domain.RecurringSchedule{
			Weekdays: weekdays(t, time.Monday, time.Wednesday), Duration: time.Hour, Period: domain.Morning,
		}},
		"recurring without period": {Title: "Leitura", Priority: domain.Low, Schedule: domain.RecurringSchedule{
			Weekdays: weekdays(t, time.Sunday), Duration: 30 * time.Minute,
		}},
		"one-off": {Title: "Relatório", Priority: domain.Medium, Schedule: domain.OneOffSchedule{
			Duration: 3 * time.Hour, Deadline: calendar.NewDate(2026, time.October, 30), Period: domain.Afternoon,
		}},
	}
}

func create(t *testing.T, repo *Repository, userID uuid.UUID, d domain.Details) domain.Task {
	t.Helper()
	task, err := domain.New(userID, d)
	require.NoError(t, err)
	created, err := repo.Create(t.Context(), task)
	require.NoError(t, err)
	return created
}

func TestCreateAndGetKeepEveryKind(t *testing.T) {
	for name, d := range details(t) {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			created := create(t, repo, userA, d)

			got, err := repo.Get(t.Context(), userA, created.ID)

			require.NoError(t, err)
			assert.Equal(t, d, got.Details)
			assert.Equal(t, userA, got.UserID)
			assert.False(t, got.CreatedAt.IsZero())
			assert.True(t, got.CreatedAt.Equal(created.CreatedAt))
		})
	}
}

func TestGetHidesOtherUsersTasks(t *testing.T) {
	repo := newRepo(t)
	task := create(t, repo, userA, details(t)["recurring"])

	_, err := repo.Get(t.Context(), userB, task.ID)

	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
}

func TestListReturnsOnlyTheUsersTasksNewestFirst(t *testing.T) {
	repo := newRepo(t)
	d := details(t)
	first := create(t, repo, userA, d["recurring"])
	second := create(t, repo, userA, d["one-off"])
	third := create(t, repo, userA, d["fixed weekly"])
	create(t, repo, userB, d["recurring"])

	page, total, err := repo.List(t.Context(), userA, 0, 2)
	require.NoError(t, err)
	rest, _, err := repo.List(t.Context(), userA, 2, 2)
	require.NoError(t, err)

	assert.Equal(t, 3, total)
	assert.Equal(t, []uuid.UUID{third.ID, second.ID}, ids(page))
	assert.Equal(t, []uuid.UUID{first.ID}, ids(rest))
}

func ids(tasks []domain.Task) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.ID)
	}
	return out
}

func TestUpdateChangesKindAndClearsOldFields(t *testing.T) {
	repo := newRepo(t)
	d := details(t)
	task := create(t, repo, userA, d["recurring"])
	require.NoError(t, task.Update(d["fixed on date"]))

	updated, err := repo.Update(t.Context(), task)
	require.NoError(t, err)
	got, err := repo.Get(t.Context(), userA, task.ID)

	require.NoError(t, err)
	assert.Equal(t, d["fixed on date"], got.Details)
	assert.True(t, got.CreatedAt.Equal(task.CreatedAt))
	assert.True(t, updated.UpdatedAt.After(task.UpdatedAt))
}

func TestUpdateIgnoresOtherUsersTasks(t *testing.T) {
	repo := newRepo(t)
	d := details(t)
	task := create(t, repo, userA, d["recurring"])
	forged := task
	forged.UserID = userB
	require.NoError(t, forged.Update(d["one-off"]))

	_, err := repo.Update(t.Context(), forged)
	got, getErr := repo.Get(t.Context(), userA, task.ID)

	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
	require.NoError(t, getErr)
	assert.Equal(t, d["recurring"], got.Details)
}

func TestDelete(t *testing.T) {
	repo := newRepo(t)
	task := create(t, repo, userA, details(t)["recurring"])

	require.ErrorIs(t, repo.Delete(t.Context(), userB, task.ID), domain.ErrTaskNotFound)
	_, err := repo.Get(t.Context(), userA, task.ID)
	require.NoError(t, err, "a task of another user must survive")

	require.NoError(t, repo.Delete(t.Context(), userA, task.ID))
	_, err = repo.Get(t.Context(), userA, task.ID)
	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
	assert.ErrorIs(t, repo.Delete(t.Context(), userA, task.ID), domain.ErrTaskNotFound)
}

func TestDatabaseRejectsInconsistentRows(t *testing.T) {
	tx := dbtest.Tx(t)
	m := toModel(domain.Task{ID: uuid.New(), UserID: userA, Details: details(t)["fixed weekly"]})
	m.Priority = new("high")

	err := tx.Create(&m).Error

	assert.ErrorContains(t, err, "tasks_fixed_shape")
}
