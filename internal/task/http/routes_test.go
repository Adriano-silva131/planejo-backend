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

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/db/dbtest"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
	"github.com/Adriano-silva131/planejo-backend/internal/task/app"
	"github.com/Adriano-silva131/planejo-backend/internal/task/infra"
)

const (
	tokenA = "token-a"
	tokenB = "token-b"
)

func TestMain(m *testing.M) {
	dbtest.Main(m)
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (auth.User, error) {
	switch token {
	case tokenA:
		return auth.User{ID: uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90")}, nil
	case tokenB:
		return auth.User{ID: uuid.MustParse("0b9c3f0e-2222-4a8e-9a5b-2f1d7c3e8b90")}, nil
	}
	return auth.User{}, auth.ErrInvalidToken
}

type server struct {
	t       *testing.T
	handler http.Handler
}

func newServer(t *testing.T) server {
	t.Helper()
	router := httpx.New(httpx.Options{
		AllowedOrigins:           []string{"http://localhost:5173"},
		RequestTimeout:           5 * time.Second,
		Verifier:                 fakeVerifier{},
		PublicRateLimit:          1000,
		AuthenticatedIPRateLimit: 1000,
		UserRateLimit:            1000,
	}, slog.New(slog.DiscardHandler))
	Register(router.Authenticated, app.NewTasks(infra.NewRepository(dbtest.Tx(t))))
	return server{t: t, handler: router.Handler}
}

func (s server) do(method, path, token, body string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequestWithContext(s.t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func (s server) create(token, body string) map[string]any {
	s.t.Helper()
	rec := s.do(http.MethodPost, "/v1/tasks", token, body)
	require.Equal(s.t, http.StatusCreated, rec.Code, rec.Body.String())
	return decode(s.t, rec)
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body, ok := decode(t, rec)["error"].(map[string]any)
	require.True(t, ok, rec.Body.String())
	return body
}

const (
	gymJSON = `{"title":"Academia","area":"Saúde","kind":"recurring","priority":"high",
		"weekdays":["monday","wednesday","friday"],"durationMinutes":60,"period":"morning"}`
	workJSON = `{"title":"Trabalho","kind":"fixed",
		"weekdays":["monday","tuesday","wednesday","thursday","friday"],"start":"09:00","end":"18:00"}`
	appointmentJSON = `{"title":"Consulta médica","area":"Saúde","kind":"fixed",
		"date":"2026-11-05","start":"15:00","end":"16:00"}`
	reportJSON = `{"title":"Entregar relatório","area":"Trabalho","kind":"one_off","priority":"medium",
		"durationMinutes":180,"deadline":"2026-10-30","period":"afternoon"}`
)

func TestCreateEachKind(t *testing.T) {
	for name, body := range map[string]string{
		"recurring": gymJSON, "fixed weekly": workJSON, "fixed on date": appointmentJSON, "one-off": reportJSON,
	} {
		t.Run(name, func(t *testing.T) {
			created := newServer(t).create(tokenA, body)

			var sent map[string]any
			require.NoError(t, json.Unmarshal([]byte(body), &sent))
			for field, value := range sent {
				assert.Equal(t, value, created[field], field)
			}
			assert.NotEmpty(t, created["id"])
			assert.NotEmpty(t, created["createdAt"])
		})
	}
}

func TestCreateReportsFieldsByKind(t *testing.T) {
	rec := newServer(t).do(http.MethodPost, "/v1/tasks", tokenA,
		`{"title":"Academia","kind":"recurring","weekdays":["monday"],"deadline":"2026-10-30"}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, map[string]any{
		"priority":        []any{"Obrigatório para este tipo de tarefa."},
		"durationMinutes": []any{"Obrigatório para este tipo de tarefa."},
		"deadline":        []any{"Não se aplica a este tipo de tarefa."},
	}, errorOf(t, rec)["fields"])
}

func TestCreateReportsDomainRules(t *testing.T) {
	for want, body := range map[string]string{
		"fixed_task_days": `{"title":"Trabalho","kind":"fixed","weekdays":["monday"],
			"date":"2026-11-05","start":"09:00","end":"18:00"}`,
		"task_invalid_time_range": `{"title":"Trabalho","kind":"fixed","weekdays":["monday"],
			"start":"18:00","end":"09:00"}`,
		"task_title_required": `{"title":"   ","kind":"one_off","priority":"low",
			"durationMinutes":30,"deadline":"2026-10-30"}`,
	} {
		t.Run(want, func(t *testing.T) {
			rec := newServer(t).do(http.MethodPost, "/v1/tasks", tokenA, body)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			assert.Equal(t, want, errorOf(t, rec)["code"])
		})
	}
}

func TestCreateValidatesFormats(t *testing.T) {
	rec := newServer(t).do(http.MethodPost, "/v1/tasks", tokenA,
		`{"title":"Trabalho","kind":"fixed","weekdays":["mon"],"start":"9h","end":"18:00"}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	fields := errorOf(t, rec)["fields"]
	assert.Contains(t, fields, "start")
	assert.Contains(t, fields, "weekdays[0]")
}

func TestGetUpdateAndDelete(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.create(tokenA, gymJSON)["id"].(string)

	got := srv.do(http.MethodGet, path, tokenA, "")
	require.Equal(t, http.StatusOK, got.Code)
	assert.Equal(t, "Academia", decode(t, got)["title"])

	updated := srv.do(http.MethodPut, path, tokenA, reportJSON)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	body := decode(t, updated)
	assert.Equal(t, "one_off", body["kind"])
	assert.NotContains(t, body, "weekdays")

	assert.Equal(t, http.StatusNoContent, srv.do(http.MethodDelete, path, tokenA, "").Code)
	assert.Equal(t, http.StatusNotFound, srv.do(http.MethodGet, path, tokenA, "").Code)
}

func TestListPaginates(t *testing.T) {
	srv := newServer(t)
	for _, body := range []string{gymJSON, workJSON, reportJSON} {
		srv.create(tokenA, body)
	}
	srv.create(tokenB, appointmentJSON)

	rec := srv.do(http.MethodGet, "/v1/tasks?page=2&perPage=2", tokenA, "")

	require.Equal(t, http.StatusOK, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, map[string]any{"total": 3.0, "page": 2.0, "perPage": 2.0}, body["meta"])
	data, ok := body["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	assert.Equal(t, "Academia", data[0].(map[string]any)["title"])
}

func TestOtherUsersTasksAreNotFound(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.create(tokenA, gymJSON)["id"].(string)

	for name, method := range map[string]string{"get": http.MethodGet, "update": http.MethodPut, "delete": http.MethodDelete} {
		t.Run(name, func(t *testing.T) {
			rec := srv.do(method, path, tokenB, reportJSON)

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, "task_not_found", errorOf(t, rec)["code"])
		})
	}

	still := srv.do(http.MethodGet, path, tokenA, "")
	require.Equal(t, http.StatusOK, still.Code)
	assert.Equal(t, "Academia", decode(t, still)["title"])
}

func TestTasksRequireToken(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, newServer(t).do(http.MethodGet, "/v1/tasks", "", "").Code)
}
