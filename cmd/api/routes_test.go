package main

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
)

func TestMain(m *testing.M) {
	dbtest.Main(m)
}

type oneUser struct{}

func (oneUser) Verify(context.Context, string) (auth.User, error) {
	return auth.User{ID: uuid.MustParse("0b9c3f0e-1111-4a8e-9a5b-2f1d7c3e8b90")}, nil
}

// api serves the routes wired by registerRoutes, as in production, on a
// transaction rolled back at the end of the test.
func api(t *testing.T) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	call, _ := apiOn(t)
	return call
}

func apiOn(t *testing.T) (func(method, path, body string) *httptest.ResponseRecorder, *gorm.DB) {
	t.Helper()
	router := httpx.New(httpx.Options{
		AllowedOrigins:           []string{"http://localhost:5173"},
		RequestTimeout:           5 * time.Second,
		Verifier:                 oneUser{},
		PublicRateLimit:          1000,
		AuthenticatedIPRateLimit: 1000,
		UserRateLimit:            1000,
	}, slog.New(slog.DiscardHandler))
	tx := dbtest.Tx(t)
	registerRoutes(router, tx)

	return func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Time-Zone", "America/Sao_Paulo")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.Handler.ServeHTTP(rec, req)
		return rec
	}, tx
}

func mondayBlocks(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var week struct {
		Days []struct {
			Blocks []any `json:"blocks"`
		} `json:"days"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &week))
	return len(week.Days[0].Blocks)
}

func TestChangingTheScheduleDropsAdjustmentsThatNoLongerApply(t *testing.T) {
	call := api(t)
	const onMondays = `{"title":"Faculdade","area":"Estudos","kind":"fixed","weekdays":["monday"],"start":"19:00","end":"22:00"}`
	created := call(http.MethodPost, "/v1/tasks", onMondays)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &task))
	// A Monday far ahead, so the creation date never hides it.
	occurrence := "/v1/tasks/" + task.ID + "/occurrences/2030-01-07"
	require.Equal(t, http.StatusOK, call(http.MethodPut, occurrence, `{"skipped":true}`).Code)
	require.Zero(t, mondayBlocks(t, call(http.MethodGet, "/v1/routine?week=2030-01-07", "")))

	moved := call(http.MethodPut, "/v1/tasks/"+task.ID, strings.Replace(onMondays, "monday", "tuesday", 1))
	require.Equal(t, http.StatusOK, moved.Code, moved.Body.String())
	back := call(http.MethodPut, "/v1/tasks/"+task.ID, onMondays)
	require.Equal(t, http.StatusOK, back.Code, back.Body.String())

	assert.Equal(t, 1, mondayBlocks(t, call(http.MethodGet, "/v1/routine?week=2030-01-07", "")),
		"the old skip does not come back")
	assert.Equal(t, http.StatusNotFound, call(http.MethodDelete, occurrence, "").Code)
}

func TestChangingTheKindDropsAdjustments(t *testing.T) {
	call := api(t)
	const fixed = `{"title":"Inglês","area":"Estudos","kind":"fixed","weekdays":["monday"],"start":"19:00","end":"20:00"}`
	created := call(http.MethodPost, "/v1/tasks", fixed)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &task))
	occurrence := "/v1/tasks/" + task.ID + "/occurrences/2030-01-07"
	require.Equal(t, http.StatusOK, call(http.MethodPut, occurrence, `{"start":"21:00","end":"22:00"}`).Code)

	recurring := call(http.MethodPut, "/v1/tasks/"+task.ID, recurringEnglish)
	require.Equal(t, http.StatusOK, recurring.Code, recurring.Body.String())

	assert.Zero(t, mondayBlocks(t, call(http.MethodGet, "/v1/routine?week=2030-01-07", "")),
		"the move made to the fixed task does not apply to the recurring one")
	back := call(http.MethodPut, "/v1/tasks/"+task.ID, fixed)
	require.Equal(t, http.StatusOK, back.Code, back.Body.String())
	assert.Equal(t, http.StatusNotFound, call(http.MethodDelete, occurrence, "").Code)
}

const recurringEnglish = `{"title":"Inglês","area":"Estudos","kind":"recurring","priority":"medium",
	"timesPerWeek":2,"durationMinutes":60}`

func TestBecomingFixedDropsSessions(t *testing.T) {
	call := api(t)
	created := call(http.MethodPost, "/v1/tasks", recurringEnglish)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &task))
	session := "/v1/tasks/" + task.ID + "/sessions/2030-01-07"
	require.Equal(t, http.StatusOK, call(http.MethodPut, session, `{"start":"07:00"}`).Code)
	require.Equal(t, 1, mondayBlocks(t, call(http.MethodGet, "/v1/routine?week=2030-01-07", "")))

	fixed := call(http.MethodPut, "/v1/tasks/"+task.ID,
		`{"title":"Inglês","area":"Estudos","kind":"fixed","weekdays":["tuesday"],"start":"19:00","end":"20:00"}`)
	require.Equal(t, http.StatusOK, fixed.Code, fixed.Body.String())

	assert.Zero(t, mondayBlocks(t, call(http.MethodGet, "/v1/routine?week=2030-01-07", "")))
	assert.Equal(t, http.StatusNotFound, call(http.MethodDelete, session, "").Code)
}

func TestEveryModuleIsWired(t *testing.T) {
	call := api(t)

	for _, path := range []string{
		"/v1/tasks", "/v1/tasks/areas", "/v1/progress?week=2026-10-26",
		"/v1/progress/habits?week=2026-10-26", "/v1/routine?week=2026-10-26",
	} {
		assert.Equal(t, http.StatusOK, call(http.MethodGet, path, "").Code, path)
	}
}

func TestChangingAHabitsRecurrenceKeepsTheOldOne(t *testing.T) {
	call, tx := apiOn(t)
	const threeDays = `{"title":"Academia","area":"Saúde","kind":"recurring","priority":"high",
		"timesPerWeek":3,"weekdays":["monday","wednesday","friday"],"durationMinutes":60}`
	created := call(http.MethodPost, "/v1/tasks", threeDays)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &task))

	longer := strings.Replace(threeDays, `"durationMinutes":60`, `"durationMinutes":90`, 1)
	require.Equal(t, http.StatusOK, call(http.MethodPut, "/v1/tasks/"+task.ID, longer).Code)
	fiveTimes := strings.Replace(threeDays, `"timesPerWeek":3`, `"timesPerWeek":5`, 1)
	require.Equal(t, http.StatusOK, call(http.MethodPut, "/v1/tasks/"+task.ID, fiveTimes).Code)

	var kept []struct {
		Weekdays     int
		TimesPerWeek int
	}
	require.NoError(t, tx.Raw("SELECT weekdays, times_per_week FROM habit_schedules WHERE task_id = ?", task.ID).
		Scan(&kept).Error)
	assert.Equal(t, []struct {
		Weekdays     int
		TimesPerWeek int
	}{{1<<1 | 1<<3 | 1<<5, 3}}, kept, "only the recurrence change is kept, with the old one")
}
