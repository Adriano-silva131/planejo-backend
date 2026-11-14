package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// wednesday is "now" in the tests: 2026-10-28 at noon, UTC.
var wednesday = time.Date(2026, time.October, 28, 12, 0, 0, 0, time.UTC)

// fakeProgress stands in for the progress module, which main wires in
// production.
type fakeProgress struct {
	habits    []domain.Habit
	checkIns  []domain.CheckIn
	completed map[uuid.UUID]bool
}

func (p *fakeProgress) Habits(context.Context, uuid.UUID, calendar.Date) ([]domain.Habit, error) {
	return p.habits, nil
}

func (p *fakeProgress) CheckIns(context.Context, uuid.UUID, calendar.Date, calendar.Date) ([]domain.CheckIn, error) {
	return p.checkIns, nil
}

func (p *fakeProgress) Completed(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]bool, error) {
	return p.completed, nil
}

func (s server) doWithTimeZone(path, zone string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequestWithContext(s.t.Context(), http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer token-a")
	req.Header.Set("Time-Zone", zone)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func dayOf(t *testing.T, week map[string]any, i int) map[string]any {
	t.Helper()
	days, ok := week["days"].([]any)
	require.True(t, ok)
	day, ok := days[i].(map[string]any)
	require.True(t, ok)
	return day
}

func toPlaceOf(t *testing.T, week map[string]any, taskID string) []any {
	t.Helper()
	list, ok := week["toPlace"].([]any)
	require.True(t, ok)
	out := []any{}
	for _, item := range list {
		if m, _ := item.(map[string]any); m["taskId"] == taskID {
			out = append(out, m)
		}
	}
	return out
}

func TestWeekRoutinePlansWhatHasNoTime(t *testing.T) {
	srv := newServer(t)
	english := gym()
	english.Title, english.Period = "Inglês", calendar.Evening
	englishID := srv.addPlannable(userA, english)
	reportID := srv.addPlannable(userA, domain.PlannableTask{
		Title: "Relatório", Area: "Trabalho", Kind: domain.OneOffKind, Deadline: calendar.NewDate(2026, time.October, 30),
	})
	id := uuid.MustParse(englishID)
	srv.progress.habits = []domain.Habit{{TaskID: id, Target: 3, Weekdays: weekdays(t, time.Monday, time.Wednesday, time.Friday)}}
	srv.progress.checkIns = []domain.CheckIn{{TaskID: id, Date: calendar.NewDate(2026, time.October, 27), Start: calendar.SomeTime(7 * 60)}}
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, "/v1/tasks/"+englishID+"/sessions/2026-10-28", "token-a", `{"start":"19:00"}`).Code)

	week := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-28", "token-a", ""))

	assert.Equal(t, []any{map[string]any{
		"taskId": englishID, "title": "Inglês", "area": "Saúde", "taskKind": "recurring", "source": "checkin",
		"period": "morning", "minutes": float64(60), "done": true,
	}}, dayOf(t, week, 1)["plans"], "the check-in started in the morning")
	assert.Equal(t, []any{}, dayOf(t, week, 2)["plans"], "a session with a time is a block")
	block, _ := dayOf(t, week, 2)["blocks"].([]any)[0].(map[string]any)
	assert.Equal(t, false, block["done"])
	friday, _ := dayOf(t, week, 4)["plans"].([]any)[0].(map[string]any)
	assert.Equal(t, "usual", friday["source"])
	assert.Empty(t, toPlaceOf(t, week, englishID), "Monday was missed, but 1 done, 1 session and Friday make 3")
	assert.Equal(t, []any{map[string]any{
		"taskId": reportID, "title": "Relatório", "area": "Trabalho", "taskKind": "one_off",
		"deadline": "2026-10-30", "remaining": float64(1), "missed": float64(0),
	}}, toPlaceOf(t, week, reportID))
}

func TestWeekRoutineRefusesAnInvalidTimeZone(t *testing.T) {
	srv := newServer(t)
	path := "/v1/routine?week=2026-10-28"

	rec := srv.doWithTimeZone(path, "Marte/Olympus")

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, errorOf(t, rec)["fields"], "Time-Zone")
	assert.Equal(t, http.StatusOK, srv.doWithTimeZone(path, "America/Sao_Paulo").Code)
}
