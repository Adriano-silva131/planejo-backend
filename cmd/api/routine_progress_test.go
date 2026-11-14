package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type plannedWeek struct {
	Days []struct {
		Plans []struct {
			TaskID string `json:"taskId"`
			Source string `json:"source"`
			Period string `json:"period"`
		} `json:"plans"`
	} `json:"days"`
	ToPlace []struct {
		TaskID    string `json:"taskId"`
		Remaining int    `json:"remaining"`
	} `json:"toPlace"`
}

func TestTheRoutinePlansFromTheProgress(t *testing.T) {
	call := api(t)
	english := createTask(t, call, `{"title":"Inglês","area":"Estudos","kind":"recurring","priority":"high",
		"timesPerWeek":2,"weekdays":["monday","wednesday","friday"],"durationMinutes":60,"period":"evening"}`)
	report := createTask(t, call, `{"title":"Relatório","area":"Trabalho","kind":"one_off","priority":"low","deadline":"2030-01-10"}`)
	// A week far ahead, so the creation date never hides it.
	const ahead = "/v1/routine?week=2030-01-07"

	w := routineWeek(t, call, ahead)

	require.Len(t, w.Days[0].Plans, 1)
	assert.Equal(t, english, w.Days[0].Plans[0].TaskID)
	assert.Equal(t, "usual", w.Days[0].Plans[0].Source)
	assert.Equal(t, "evening", w.Days[0].Plans[0].Period)
	assert.Len(t, w.Days[2].Plans, 1)
	assert.Empty(t, w.Days[4].Plans, "the week asks for two days")
	require.Len(t, w.ToPlace, 1)
	assert.Equal(t, report, w.ToPlace[0].TaskID)

	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	today := calendar.DateOf(time.Now().In(loc)).String()
	done := call(http.MethodPut, "/v1/tasks/"+report+"/checkins/"+today, "")
	require.Equal(t, http.StatusOK, done.Code, done.Body.String())

	assert.Empty(t, routineWeek(t, call, ahead).ToPlace, "the one-off task is completed")
}

func createTask(t *testing.T, call func(method, path, body string) *httptest.ResponseRecorder, body string) string {
	t.Helper()
	rec := call(http.MethodPost, "/v1/tasks", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &task))
	return task.ID
}

func routineWeek(t *testing.T, call func(method, path, body string) *httptest.ResponseRecorder, path string) plannedWeek {
	t.Helper()
	rec := call(http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var w plannedWeek
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	return w
}
