package infra

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
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

// task inserts a fixed task straight into the table, without the task module.
func (f fixture) task(userID uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	require.NoError(f.t, f.tx.Exec(
		`INSERT INTO tasks (id, user_id, title, area, kind, weekdays, start_minute, end_minute)
		 VALUES (?, ?, 'Trabalho', 'Trabalho', 'fixed', 62, 540, 1080)`, id, userID).Error)
	return id
}

func (f fixture) list(userID uuid.UUID) []domain.Adjustment {
	f.t.Helper()
	adjustments, err := f.repo.List(f.t.Context(), userID, monday.AddDays(-1), monday.AddDays(6))
	require.NoError(f.t, err)
	return adjustments
}

func moved(userID, taskID uuid.UUID, date calendar.Date) domain.Adjustment {
	return domain.Adjustment{UserID: userID, TaskID: taskID, Date: date, Start: 9 * 60, End: 16 * 60}
}

func TestSaveSkippedAndMoved(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)

	_, err := f.repo.Save(t.Context(), domain.Adjustment{UserID: userA, TaskID: task, Date: monday, Skipped: true})
	require.NoError(t, err)
	saved, err := f.repo.Save(t.Context(), moved(userA, task, monday.AddDays(4)))
	require.NoError(t, err)

	got := f.list(userA)
	require.Len(t, got, 2)
	assert.True(t, got[0].Skipped)
	assert.Equal(t, monday, got[0].Date)
	assert.Equal(t, calendar.TimeOfDay(16*60), got[1].End)
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestSaveReplacesTheSameOccurrence(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	first, err := f.repo.Save(t.Context(), moved(userA, task, monday))
	require.NoError(t, err)

	second, err := f.repo.Save(t.Context(), domain.Adjustment{UserID: userA, TaskID: task, Date: monday, Skipped: true})

	require.NoError(t, err)
	got := f.list(userA)
	require.Len(t, got, 1)
	assert.True(t, got[0].Skipped)
	assert.Zero(t, got[0].Start, "the old times are cleared")
	assert.True(t, second.CreatedAt.Equal(first.CreatedAt))
}

func TestSaveRejectsTasksOfAnotherUser(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	_, err := f.repo.Save(t.Context(), moved(userA, task, monday))
	require.NoError(t, err)

	_, overwriteErr := f.repo.Save(t.Context(), domain.Adjustment{UserID: userB, TaskID: task, Date: monday, Skipped: true})
	assert.ErrorIs(t, overwriteErr, domain.ErrTaskNotFound)
	assert.False(t, f.list(userA)[0].Skipped, "the owner's adjustment must survive")

	// Last: the foreign key violation aborts the test's transaction.
	_, newErr := f.repo.Save(t.Context(), moved(userB, task, monday.AddDays(1)))
	assert.ErrorIs(t, newErr, domain.ErrTaskNotFound)
}

func TestListOnlyThePeriodAndTheUser(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	for _, date := range []calendar.Date{monday.AddDays(-2), monday.AddDays(-1), monday.AddDays(6), monday.AddDays(7)} {
		_, err := f.repo.Save(t.Context(), moved(userA, task, date))
		require.NoError(t, err)
	}
	_, err := f.repo.Save(t.Context(), moved(userB, f.task(userB), monday))
	require.NoError(t, err)

	got := f.list(userA)

	require.Len(t, got, 2)
	assert.Equal(t, monday.AddDays(-1), got[0].Date)
	assert.Equal(t, monday.AddDays(6), got[1].Date)
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	_, err := f.repo.Save(t.Context(), moved(userA, task, monday))
	require.NoError(t, err)

	require.ErrorIs(t, f.repo.Delete(t.Context(), userB, task, monday), domain.ErrAdjustmentNotFound)
	require.Len(t, f.list(userA), 1)

	require.NoError(t, f.repo.Delete(t.Context(), userA, task, monday))
	assert.Empty(t, f.list(userA))
	assert.ErrorIs(t, f.repo.Delete(t.Context(), userA, task, monday), domain.ErrAdjustmentNotFound)
}

func TestDeletingTheTaskRemovesItsAdjustments(t *testing.T) {
	f := newFixture(t)
	task := f.task(userA)
	_, err := f.repo.Save(t.Context(), moved(userA, task, monday))
	require.NoError(t, err)

	require.NoError(t, f.tx.Exec("DELETE FROM tasks WHERE id = ?", task).Error)

	assert.Empty(t, f.list(userA))
}

func TestForTask(t *testing.T) {
	f := newFixture(t)
	task, other := f.task(userA), f.task(userA)
	for _, date := range []calendar.Date{monday.AddDays(30), monday} {
		_, err := f.repo.Save(t.Context(), moved(userA, task, date))
		require.NoError(t, err)
	}
	_, err := f.repo.Save(t.Context(), moved(userA, other, monday))
	require.NoError(t, err)

	mine, err := f.repo.ForTask(t.Context(), userA, task)
	require.NoError(t, err)
	none, err := f.repo.ForTask(t.Context(), userB, task)
	require.NoError(t, err)

	require.Len(t, mine, 2)
	assert.Equal(t, monday, mine[0].Date)
	assert.Empty(t, none)
}
