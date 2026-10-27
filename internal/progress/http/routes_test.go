package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/infra"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"
)

var (
	userA = uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90")
	userB = uuid.MustParse("0b9c3f0e-2222-4a8e-9a5b-2f1d7c3e8b90")
)

func TestMain(m *testing.M) {
	dbtest.Main(m)
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (auth.User, error) {
	switch token {
	case "token-a":
		return auth.User{ID: userA}, nil
	case "token-b":
		return auth.User{ID: userB}, nil
	}
	return auth.User{}, auth.ErrInvalidToken
}

type ownedTask struct {
	owner uuid.UUID
	task  domain.Task
}

// fakeTasks stands in for the task module, which main wires in production.
type fakeTasks map[uuid.UUID]ownedTask

func (f fakeTasks) Get(_ context.Context, userID, id uuid.UUID) (domain.Task, error) {
	t, ok := f[id]
	if !ok || t.owner != userID {
		return domain.Task{}, domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")
	}
	return t.task, nil
}

func (f fakeTasks) List(_ context.Context, userID uuid.UUID) ([]domain.Task, error) {
	var out []domain.Task
	for _, t := range f {
		if t.owner == userID {
			out = append(out, t.task)
		}
	}
	return out, nil
}

type server struct {
	t       *testing.T
	tx      *gorm.DB
	tasks   fakeTasks
	handler http.Handler
}

func newServer(t *testing.T) server {
	t.Helper()
	tx := dbtest.Tx(t)
	tasks := fakeTasks{}
	router := httpx.New(httpx.Options{
		AllowedOrigins:           []string{"http://localhost:5173"},
		RequestTimeout:           5 * time.Second,
		Verifier:                 fakeVerifier{},
		PublicRateLimit:          1000,
		AuthenticatedIPRateLimit: 1000,
		UserRateLimit:            1000,
	}, slog.New(slog.DiscardHandler))
	Register(router.Authenticated, app.NewProgress(tasks, infra.NewRepository(tx)))
	return server{t: t, tx: tx, tasks: tasks, handler: router.Handler}
}

// addTask stores a row in tasks, which check-ins reference, and exposes it
// through the fake task reader.
func (s server) addTask(owner uuid.UUID, task domain.Task) string {
	s.t.Helper()
	task.ID = uuid.New()
	require.NoError(s.t, s.tx.Exec(
		`INSERT INTO tasks (id, user_id, title, kind, start_minute, end_minute, weekdays)
		 VALUES (?, ?, ?, 'fixed', 540, 600, 2)`, task.ID, owner, task.Title).Error)
	s.tasks[task.ID] = ownedTask{owner: owner, task: task}
	return task.ID.String()
}

func (s server) do(method, path, token, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.doIn("America/Sao_Paulo", method, path, token, body)
}

// doIn sends zone as the Time-Zone header; empty sends none.
func (s server) doIn(zone, method, path, token, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequestWithContext(s.t.Context(), method, path, strings.NewReader(body))
	if zone != "" {
		req.Header.Set("Time-Zone", zone)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) any {
	t.Helper()
	e, ok := decode(t, rec)["error"].(map[string]any)
	require.True(t, ok, rec.Body.String())
	return e["code"]
}

var (
	gym    = domain.Task{Title: "Academia", Kind: domain.RecurringTask, TimesPerWeek: 3}
	work   = domain.Task{Title: "Trabalho", Kind: domain.FixedTask}
	thesis = domain.Task{
		Title: "TCC", Kind: domain.OneOffTask, TotalMinutes: 1200,
		Deadline: calendar.NewDate(2026, time.December, 10),
	}
)

func TestCheckInRecurringWithoutBody(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addTask(userA, gym) + "/checkins/2026-10-20"

	rec := srv.do(http.MethodPut, path, "token-a", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	assert.Equal(t, "2026-10-20", body["date"])
	assert.Equal(t, 0.0, body["minutes"])
	assert.Equal(t, false, body["completed"])
}

func TestCheckInOneOffWithMinutesAndHistory(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, thesis)
	base := "/v1/tasks/" + id + "/checkins/"

	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, base+"2026-10-20", "token-a", `{"minutes":90}`).Code)
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, base+"2026-10-22", "token-a", `{"minutes":60,"completed":true}`).Code)
	rec := srv.do(http.MethodGet, "/v1/tasks/"+id+"/checkins?from=2026-10-21", "token-a", "")

	require.Equal(t, http.StatusOK, rec.Code)
	data, ok := decode(t, rec)["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	assert.Equal(t, map[string]any{"date": "2026-10-22", "minutes": 60.0, "completed": true, "taskId": id},
		pick(data[0], "date", "minutes", "completed", "taskId"))
}

func pick(v any, keys ...string) map[string]any {
	m, _ := v.(map[string]any)
	out := map[string]any{}
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

func TestCheckInRules(t *testing.T) {
	srv := newServer(t)
	gymID, workID, thesisID := srv.addTask(userA, gym), srv.addTask(userA, work), srv.addTask(userA, thesis)

	cases := map[string]struct {
		path, body string
		want       string
	}{
		"fixed task":           {workID + "/checkins/2026-10-20", "", "fixed_task_checkin"},
		"future day":           {gymID + "/checkins/2030-01-01", "", "future_checkin"},
		"minutes on recurring": {gymID + "/checkins/2026-10-20", `{"minutes":30}`, "checkin_one_off_only"},
		"too many minutes":     {thesisID + "/checkins/2026-10-20", `{"minutes":2000}`, "validation_failed"},
		"invalid date":         {gymID + "/checkins/20-10-2026", "", "validation_failed"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := srv.do(http.MethodPut, "/v1/tasks/"+tc.path, "token-a", tc.body)

			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			assert.Equal(t, tc.want, errorCode(t, rec))
		})
	}
}

func TestCheckInRequiresAValidTimeZone(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addTask(userA, gym) + "/checkins/2026-10-20"

	for name, zone := range map[string]string{"missing": "", "unknown": "Mars/Olympus", "server zone": "Local"} {
		t.Run(name, func(t *testing.T) {
			rec := srv.doIn(zone, http.MethodPut, path, "token-a", "")

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			e, ok := decode(t, rec)["error"].(map[string]any)
			require.True(t, ok)
			assert.Contains(t, e["fields"], "Time-Zone")
		})
	}
}

func TestUndoCheckIn(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addTask(userA, gym) + "/checkins/2026-10-20"
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", "").Code)

	assert.Equal(t, http.StatusNoContent, srv.do(http.MethodDelete, path, "token-a", "").Code)
	again := srv.do(http.MethodDelete, path, "token-a", "")
	assert.Equal(t, http.StatusNotFound, again.Code)
	assert.Equal(t, "checkin_not_found", errorCode(t, again))
}

func TestOtherUsersTasksAreNotFound(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, gym)
	path := "/v1/tasks/" + id + "/checkins/2026-10-20"
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", "").Code)

	put := srv.do(http.MethodPut, path, "token-b", "")
	history := srv.do(http.MethodGet, "/v1/tasks/"+id+"/checkins", "token-b", "")
	undo := srv.do(http.MethodDelete, path, "token-b", "")

	assert.Equal(t, "task_not_found", errorCode(t, put))
	assert.Equal(t, "task_not_found", errorCode(t, history))
	assert.Equal(t, http.StatusNotFound, undo.Code)
	still := srv.do(http.MethodGet, "/v1/tasks/"+id+"/checkins", "token-a", "")
	assert.Len(t, decode(t, still)["data"], 1, "the owner's check-in must survive")
}

func TestWeekProgress(t *testing.T) {
	srv := newServer(t)
	gymID, thesisID := srv.addTask(userA, gym), srv.addTask(userA, thesis)
	srv.addTask(userA, work)
	for _, day := range []string{"2026-10-20", "2026-10-21", "2026-10-26"} {
		require.Equal(t, http.StatusOK, srv.do(http.MethodPut, "/v1/tasks/"+gymID+"/checkins/"+day, "token-a", "").Code)
	}
	for day, minutes := range map[string]string{"2026-10-15": "120", "2026-10-22": "90"} {
		require.Equal(t, http.StatusOK,
			srv.do(http.MethodPut, "/v1/tasks/"+thesisID+"/checkins/"+day, "token-a", `{"minutes":`+minutes+`}`).Code)
	}

	rec := srv.do(http.MethodGet, "/v1/progress?week=2026-10-22", "token-a", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	assert.Equal(t, "2026-10-19", body["start"])
	assert.Equal(t, "2026-10-25", body["end"])
	assert.Equal(t, []any{map[string]any{
		"taskId": gymID, "title": "Academia", "expected": 3.0, "done": 2.0,
		"days": []any{"2026-10-20", "2026-10-21"},
	}}, body["recurring"])
	assert.Equal(t, []any{map[string]any{
		"taskId": thesisID, "title": "TCC", "totalMinutes": 1200.0, "deadline": "2026-12-10",
		"weekMinutes": 90.0, "doneMinutes": 210.0,
	}}, body["oneOff"])
}

func TestWeekRequiresTheWeek(t *testing.T) {
	rec := newServer(t).do(http.MethodGet, "/v1/progress", "token-a", "")

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
