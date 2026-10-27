package infra

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

var (
	userA  = uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90")
	userB  = uuid.MustParse("0b9c3f0e-2222-4a8e-9a5b-2f1d7c3e8b90")
	monday = calendar.NewDate(2026, time.October, 26)
)

func TestMain(m *testing.M) {
	dbtest.Main(m)
}

type fixture struct {
	t    *testing.T
	tx   *gorm.DB
	repo *Repository
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	tx := dbtest.Tx(t)
	return fixture{t: t, tx: tx, repo: NewRepository(tx)}
}

// task inserts a recurring task straight into the table, without the task module.
func (f fixture) task(userID uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	require.NoError(f.t, f.tx.Exec(
		`INSERT INTO tasks (id, user_id, title, area, kind, priority, weekdays, duration_minutes)
		 VALUES (?, ?, 'Academia', 'Saúde', 'recurring', 'high', 2, 60)`, id, userID).Error)
	return id
}

func (f fixture) save(userID, taskID uuid.UUID, date calendar.Date, minutes int) domain.CheckIn {
	f.t.Helper()
	c, err := f.repo.Save(f.t.Context(), domain.CheckIn{UserID: userID, TaskID: taskID, Date: date, Minutes: minutes})
	require.NoError(f.t, err)
	return c
}

func (f fixture) list(userID uuid.UUID, filter domain.Filter) []domain.CheckIn {
	f.t.Helper()
	checkIns, err := f.repo.List(f.t.Context(), userID, filter)
	require.NoError(f.t, err)
	return checkIns
}

func TestSaveAndList(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)

	saved := f.save(userA, task, monday, 30)
	got := f.list(userA, domain.Filter{})

	require.Len(t, got, 1)
	assert.Equal(t, monday, got[0].Date)
	assert.Equal(t, 30, got[0].Minutes)
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestSaveReplacesTheSameDay(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	first := f.save(userA, task, monday, 30)

	second, err := f.repo.Save(t.Context(), domain.CheckIn{
		UserID: userA, TaskID: task, Date: monday, Minutes: 90, Completed: true,
	})

	require.NoError(t, err)
	got := f.list(userA, domain.Filter{})
	require.Len(t, got, 1)
	assert.Equal(t, 90, got[0].Minutes)
	assert.True(t, got[0].Completed)
	assert.True(t, second.CreatedAt.Equal(first.CreatedAt), "created_at is kept")
}

func TestSaveRejectsTaskOfAnotherUser(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)

	_, err := f.repo.Save(t.Context(), domain.CheckIn{UserID: userB, TaskID: task, Date: monday})

	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
}

func TestSaveOnDeletedTask(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	require.NoError(t, f.tx.Exec("DELETE FROM tasks WHERE id = ?", task).Error)

	_, err := f.repo.Save(t.Context(), domain.CheckIn{UserID: userA, TaskID: task, Date: monday})

	assert.ErrorIs(t, err, domain.ErrTaskNotFound)
}

func TestSaveNeverOverwritesAnotherUsersCheckIn(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	f.save(userA, task, monday, 30)

	_, err := f.repo.Save(t.Context(), domain.CheckIn{UserID: userB, TaskID: task, Date: monday, Minutes: 999})

	require.ErrorIs(t, err, domain.ErrTaskNotFound)
	assert.Equal(t, 30, f.list(userA, domain.Filter{})[0].Minutes)
}

func TestListFilters(t *testing.T) {
	f := newFixture(t)
	gym, reading := f.task(userA), f.task(userA)
	f.save(userA, gym, monday.AddDays(2), 0)
	f.save(userA, gym, monday, 0)
	f.save(userA, reading, monday.AddDays(1), 0)
	f.save(userA, gym, monday.AddDays(9), 0)
	f.save(userB, f.task(userB), monday, 0)

	dates := func(checkIns []domain.CheckIn) []calendar.Date {
		out := make([]calendar.Date, 0, len(checkIns))
		for _, c := range checkIns {
			out = append(out, c.Date)
		}
		return out
	}

	assert.Equal(t, []calendar.Date{monday, monday.AddDays(1), monday.AddDays(2), monday.AddDays(9)},
		dates(f.list(userA, domain.Filter{})), "only the user's, oldest first")
	assert.Equal(t, []calendar.Date{monday, monday.AddDays(2), monday.AddDays(9)},
		dates(f.list(userA, domain.Filter{TaskID: gym})))
	assert.Equal(t, []calendar.Date{monday.AddDays(1), monday.AddDays(2)},
		dates(f.list(userA, domain.Filter{From: monday.AddDays(1), To: monday.AddDays(6)})))
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	f.save(userA, task, monday, 0)

	require.ErrorIs(t, f.repo.Delete(t.Context(), userB, task, monday), domain.ErrCheckInNotFound)
	require.Len(t, f.list(userA, domain.Filter{}), 1, "a check-in of another user must survive")

	require.NoError(t, f.repo.Delete(t.Context(), userA, task, monday))
	assert.Empty(t, f.list(userA, domain.Filter{}))
	assert.ErrorIs(t, f.repo.Delete(t.Context(), userA, task, monday), domain.ErrCheckInNotFound)
}

func TestDeletingTheTaskRemovesItsCheckIns(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	f.save(userA, task, monday, 0)

	require.NoError(t, f.tx.Exec("DELETE FROM tasks WHERE id = ?", task).Error)

	assert.Empty(t, f.list(userA, domain.Filter{}))
}
