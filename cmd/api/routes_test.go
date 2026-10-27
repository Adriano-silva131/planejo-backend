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

// api serves the routes wired by registerRoutes, as in production.
func api(t *testing.T) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := httpx.New(httpx.Options{
		AllowedOrigins:           []string{"http://localhost:5173"},
		RequestTimeout:           5 * time.Second,
		Verifier:                 oneUser{},
		PublicRateLimit:          1000,
		AuthenticatedIPRateLimit: 1000,
		UserRateLimit:            1000,
	}, slog.New(slog.DiscardHandler))
	registerRoutes(router, dbtest.Tx(t))

	return func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.Handler.ServeHTTP(rec, req)
		return rec
	}
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

func TestEveryModuleIsWired(t *testing.T) {
	call := api(t)

	for _, path := range []string{
		"/v1/tasks", "/v1/tasks/areas", "/v1/progress?week=2026-10-26",
		"/v1/progress/habits?week=2026-10-26", "/v1/routine?week=2026-10-26",
	} {
		assert.Equal(t, http.StatusOK, call(http.MethodGet, path, "").Code, path)
	}
}
