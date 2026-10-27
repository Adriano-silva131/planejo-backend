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
	"github.com/Adriano-silva131/planejo-backend/internal/routine/app"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/infra"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
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
	task  domain.FixedTask
}

// fakeTasks stands in for the task module, which main wires in production.
type fakeTasks map[uuid.UUID]ownedTask

func (f fakeTasks) Fixed(_ context.Context, userID uuid.UUID, _, _ calendar.Date) ([]domain.FixedTask, error) {
	var out []domain.FixedTask
	for _, t := range f {
		if t.owner == userID {
			out = append(out, t.task)
		}
	}
	return out, nil
}

func (f fakeTasks) Get(_ context.Context, userID, id uuid.UUID) (domain.FixedTask, error) {
	t, ok := f[id]
	if !ok || t.owner != userID {
		return domain.FixedTask{}, domain.ErrTaskNotFound
	}
	return t.task, nil
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
	Register(router.Authenticated, app.NewRoutine(tasks, infra.NewRepository(tx)))
	return server{t: t, tx: tx, tasks: tasks, handler: router.Handler}
}

// addTask stores a row in tasks, which adjustments reference, and exposes it
// through the fake task reader.
func (s server) addTask(owner uuid.UUID, task domain.FixedTask) string {
	s.t.Helper()
	task.ID = uuid.New()
	require.NoError(s.t, s.tx.Exec(
		`INSERT INTO tasks (id, user_id, title, area, kind, weekdays, start_minute, end_minute)
		 VALUES (?, ?, ?, ?, 'fixed', 2, 540, 600)`, task.ID, owner, task.Title, task.Area).Error)
	s.tasks[task.ID] = ownedTask{owner: owner, task: task}
	return task.ID.String()
}

func (s server) do(method, path, token, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequestWithContext(s.t.Context(), method, path, strings.NewReader(body))
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

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	e, ok := decode(t, rec)["error"].(map[string]any)
	require.True(t, ok, rec.Body.String())
	return e
}

func weekdays(t *testing.T, days ...time.Weekday) calendar.Weekdays {
	t.Helper()
	w, err := calendar.NewWeekdays(days...)
	require.NoError(t, err)
	return w
}

func work(t *testing.T) domain.FixedTask {
	t.Helper()
	return domain.FixedTask{
		Title: "Trabalho", Area: "Trabalho", Start: 9 * 60, End: 18 * 60,
		Weekdays: weekdays(t, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
	}
}

func sleep(t *testing.T) domain.FixedTask {
	t.Helper()
	return domain.FixedTask{Title: "Dormir", Area: "Descanso", Weekdays: weekdays(t, time.Sunday), Start: 23 * 60, End: 7 * 60}
}

// blocks returns "title start-end" for each block of the day at index i.
func blocks(t *testing.T, week map[string]any, i int) []string {
	t.Helper()
	days, ok := week["days"].([]any)
	require.True(t, ok)
	day, ok := days[i].(map[string]any)
	require.True(t, ok)
	list, ok := day["blocks"].([]any)
	require.True(t, ok)
	out := []string{}
	for _, b := range list {
		m, _ := b.(map[string]any)
		out = append(out, m["title"].(string)+" "+m["start"].(string)+"-"+m["end"].(string))
	}
	return out
}

func TestWeekRoutine(t *testing.T) {
	srv := newServer(t)
	srv.addTask(userA, work(t))
	srv.addTask(userA, sleep(t))
	srv.addTask(userB, work(t))

	rec := srv.do(http.MethodGet, "/v1/routine?week=2026-10-28", "token-a", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	week := decode(t, rec)
	assert.Equal(t, "2026-10-26", week["start"])
	assert.Equal(t, "2026-11-01", week["end"])
	assert.Equal(t, []string{"Dormir 00:00-07:00", "Trabalho 09:00-18:00"}, blocks(t, week, 0))
	assert.Equal(t, []string{"Dormir 23:00-24:00"}, blocks(t, week, 6))
	assert.Empty(t, blocks(t, week, 5))

	days, _ := week["days"].([]any)
	monday, _ := days[0].(map[string]any)
	assert.Equal(t, "monday", monday["weekday"])
	first, _ := monday["blocks"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{
		"taskId": first["taskId"], "title": "Dormir", "area": "Descanso", "start": "00:00", "end": "07:00",
		"adjusted": false, "continuesFromPreviousDay": true, "continuesNextDay": false,
	}, first)
}

func TestAdjustOnlyThatWeek(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, work(t))

	skipped := srv.do(http.MethodPut, "/v1/tasks/"+id+"/occurrences/2026-10-26", "token-a", `{"skipped":true}`)
	moved := srv.do(http.MethodPut, "/v1/tasks/"+id+"/occurrences/2026-10-30", "token-a", `{"start":"09:00","end":"16:00"}`)

	require.Equal(t, http.StatusOK, skipped.Code, skipped.Body.String())
	require.Equal(t, http.StatusOK, moved.Code, moved.Body.String())
	assert.Equal(t, true, decode(t, skipped)["skipped"])
	assert.Equal(t, "16:00", decode(t, moved)["end"])

	thisWeek := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-26", "token-a", ""))
	nextWeek := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-11-02", "token-a", ""))
	assert.Empty(t, blocks(t, thisWeek, 0))
	assert.Equal(t, []string{"Trabalho 09:00-16:00"}, blocks(t, thisWeek, 4))
	assert.Equal(t, []string{"Trabalho 09:00-18:00"}, blocks(t, nextWeek, 0), "the pattern is unchanged")
	assert.Equal(t, []string{"Trabalho 09:00-18:00"}, blocks(t, nextWeek, 4))
}

func TestResetBringsThePatternBack(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, work(t))
	path := "/v1/tasks/" + id + "/occurrences/2026-10-26"
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", `{"skipped":true}`).Code)

	assert.Equal(t, http.StatusNoContent, srv.do(http.MethodDelete, path, "token-a", "").Code)
	week := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-26", "token-a", ""))
	assert.Equal(t, []string{"Trabalho 09:00-18:00"}, blocks(t, week, 0))
	again := srv.do(http.MethodDelete, path, "token-a", "")
	assert.Equal(t, "adjustment_not_found", errorOf(t, again)["code"])
}

func TestAdjustReportsFields(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addTask(userA, work(t)) + "/occurrences/2026-10-26"

	cases := map[string]struct {
		body   string
		fields map[string]any
	}{
		"skip with times": {`{"skipped":true,"start":"10:00"}`, map[string]any{"start": []any{"Não se aplica ao tirar a ocorrência."}}},
		"missing end":     {`{"start":"10:00"}`, map[string]any{"end": []any{"Obrigatório para mudar o horário; ou envie skipped."}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := srv.do(http.MethodPut, path, "token-a", tc.body)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			assert.Equal(t, tc.fields, errorOf(t, rec)["fields"])
		})
	}
}

func TestAdjustRules(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, work(t))

	saturday := srv.do(http.MethodPut, "/v1/tasks/"+id+"/occurrences/2026-10-31", "token-a", `{"skipped":true}`)
	same := srv.do(http.MethodPut, "/v1/tasks/"+id+"/occurrences/2026-10-26", "token-a", `{"start":"10:00","end":"10:00"}`)

	assert.Equal(t, "not_an_occurrence", errorOf(t, saturday)["code"])
	assert.Equal(t, "adjustment_invalid_time_range", errorOf(t, same)["code"])
}

func TestOtherUsersTasksAreNotFound(t *testing.T) {
	srv := newServer(t)
	id := srv.addTask(userA, work(t))
	path := "/v1/tasks/" + id + "/occurrences/2026-10-26"
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", `{"start":"10:00","end":"12:00"}`).Code)

	put := srv.do(http.MethodPut, path, "token-b", `{"skipped":true}`)
	reset := srv.do(http.MethodDelete, path, "token-b", "")
	week := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-26", "token-b", ""))

	assert.Equal(t, "task_not_found", errorOf(t, put)["code"])
	assert.Equal(t, http.StatusNotFound, reset.Code)
	assert.Empty(t, blocks(t, week, 0))
	own := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-26", "token-a", ""))
	assert.Equal(t, []string{"Trabalho 10:00-12:00"}, blocks(t, own, 0), "the owner's adjustment must survive")
}

func TestWeekRequiresTheWeek(t *testing.T) {
	rec := newServer(t).do(http.MethodGet, "/v1/routine", "token-a", "")

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
