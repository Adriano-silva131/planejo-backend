package infra

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type sessionFixture struct {
	fixture
	sessions *SessionRepository
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()
	tx := dbtest.Tx(t)
	return sessionFixture{fixture: fixture{t: t, tx: tx, repo: NewRepository(tx)}, sessions: NewSessionRepository(tx)}
}

// recurring inserts a recurring task straight into the table.
func (f sessionFixture) recurring(userID uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	require.NoError(f.t, f.tx.Exec(
		`INSERT INTO tasks (id, user_id, title, area, kind, priority, times_per_week, duration_minutes)
		 VALUES (?, ?, 'Academia', 'Saúde', 'recurring', 'high', 3, 60)`, id, userID).Error)
	return id
}

func (f sessionFixture) listSessions(userID uuid.UUID) []domain.Session {
	f.t.Helper()
	sessions, err := f.sessions.List(f.t.Context(), userID, monday, monday.AddDays(6))
	require.NoError(f.t, err)
	return sessions
}

func session(userID, taskID uuid.UUID, date calendar.Date, start, end calendar.TimeOfDay) domain.Session {
	return domain.Session{UserID: userID, TaskID: taskID, Date: date, Start: start, End: end}
}

func TestSaveSessionReplacesTheSameDay(t *testing.T) {
	f := newSessionFixture(t)
	task := f.recurring(userA)

	first, err := f.sessions.Save(t.Context(), session(userA, task, monday, 7*60, 8*60))
	require.NoError(t, err)
	_, err = f.sessions.Save(t.Context(), session(userA, task, monday, 22*60, domain.EndOfDay))
	require.NoError(t, err)
	_, err = f.sessions.Save(t.Context(), session(userA, task, monday.AddDays(5), 10*60, 11*60))
	require.NoError(t, err)

	got := f.listSessions(userA)
	require.Len(t, got, 2)
	assert.Equal(t, calendar.TimeOfDay(22*60), got[0].Start)
	assert.Equal(t, domain.EndOfDay, got[0].End)
	assert.Equal(t, first.CreatedAt, got[0].CreatedAt)
	assert.Equal(t, monday.AddDays(5), got[1].Date)
}

func TestSaveSessionOfAnotherUsersTaskIsNotFound(t *testing.T) {
	f := newSessionFixture(t)
	task := f.recurring(userA)
	_, err := f.sessions.Save(t.Context(), session(userA, task, monday, 7*60, 8*60))
	require.NoError(t, err)

	_, overwrite := f.sessions.Save(t.Context(), session(userB, task, monday, 9*60, 10*60))
	removed := f.sessions.Delete(t.Context(), userB, task, monday)
	assert.ErrorIs(t, overwrite, domain.ErrTaskNotFound)
	assert.ErrorIs(t, removed, domain.ErrSessionNotFound)
	assert.Equal(t, calendar.TimeOfDay(7*60), f.listSessions(userA)[0].Start, "the owner's session must survive")

	// Last: the foreign key violation aborts the test's transaction.
	_, create := f.sessions.Save(t.Context(), session(userB, task, monday.AddDays(1), 9*60, 10*60))
	assert.ErrorIs(t, create, domain.ErrTaskNotFound)
}

func TestDeleteSessions(t *testing.T) {
	f := newSessionFixture(t)
	task, other := f.recurring(userA), f.recurring(userA)
	for _, s := range []domain.Session{
		session(userA, task, monday, 7*60, 8*60),
		session(userA, task, monday.AddDays(1), 7*60, 8*60),
		session(userA, other, monday, 9*60, 10*60),
	} {
		_, err := f.sessions.Save(t.Context(), s)
		require.NoError(t, err)
	}

	require.NoError(t, f.sessions.Delete(t.Context(), userA, task, monday))
	require.NoError(t, f.sessions.DeleteForTask(t.Context(), userA, task))

	got := f.listSessions(userA)
	require.Len(t, got, 1)
	assert.Equal(t, other, got[0].TaskID)
}

func TestDatabaseRejectsSessionsPastMidnight(t *testing.T) {
	f := newSessionFixture(t)
	m := sessionModel{TaskID: f.recurring(userA), Date: monday.Time(), UserID: userA, StartMinute: new(23 * 60), EndMinute: new(60)}

	assert.Error(t, f.tx.Create(&m).Error)
}

func TestListSessionsWithoutEnd(t *testing.T) {
	f := newSessionFixture(t)
	task := f.recurring(userA)
	for _, date := range []calendar.Date{monday, monday.AddDays(1), monday.AddDays(400)} {
		_, err := f.sessions.Save(t.Context(), session(userA, task, date, 7*60, 8*60))
		require.NoError(t, err)
	}

	got, err := f.sessions.List(t.Context(), userA, monday.AddDays(1), calendar.Date{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, monday.AddDays(1), got[0].Date)
	assert.Equal(t, monday.AddDays(400), got[1].Date)
}
