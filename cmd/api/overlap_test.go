package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	workJSON = `{"title":"Olist","area":"Trabalho","kind":"fixed",
		"weekdays":["monday","tuesday","wednesday","thursday","friday"],"start":"09:00","end":"18:00"}`
	lunchJSON = `{"title":"Almoço","area":"Alimentação","kind":"fixed",
		"weekdays":["monday","tuesday","wednesday","thursday","friday"],"start":"12:00","end":"13:00"}`
	snackJSON = `{"title":"Lanche","area":"Alimentação","kind":"fixed",
		"weekdays":["monday","tuesday","wednesday","thursday","friday"],"start":"15:00","end":"15:30"}`
)

func create(t *testing.T, call func(method, path, body string) *httptest.ResponseRecorder, body string) string {
	t.Helper()
	rec := call(http.MethodPost, "/v1/tasks", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var task struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &task))
	return task.ID
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Error.Code
}

func TestAFixedTaskOnTopOfTwoLayersIsNotCreated(t *testing.T) {
	call := api(t)
	create(t, call, workJSON)
	create(t, call, lunchJSON)
	create(t, call, snackJSON)

	meeting := call(http.MethodPost, "/v1/tasks", `{"title":"Reunião de alinhamento","area":"Trabalho","kind":"fixed",
		"weekdays":["wednesday"],"start":"15:00","end":"16:00"}`)

	assert.Equal(t, http.StatusConflict, meeting.Code)
	assert.Equal(t, "schedule_overlap", errorCode(t, meeting))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(call(http.MethodGet, "/v1/tasks", "").Body.Bytes(), &list))
	assert.Equal(t, 3, list.Meta.Total, "the meeting was not saved")
}

func TestAWeeklyTaskIsCheckedAgainstSessionsOfLaterWeeks(t *testing.T) {
	call := api(t)
	create(t, call, workJSON)
	english := create(t, call, recurringEnglish)
	require.Equal(t, http.StatusOK, call(http.MethodPut, "/v1/tasks/"+english+"/sessions/2030-01-07", `{"start":"15:00"}`).Code)

	meeting := call(http.MethodPost, "/v1/tasks", `{"title":"Reunião","area":"Trabalho","kind":"fixed",
		"weekdays":["monday"],"start":"15:30","end":"16:00"}`)

	assert.Equal(t, http.StatusConflict, meeting.Code)
	assert.Contains(t, meeting.Body.String(), "seg 07/01, 15:30–16:00")
}

func TestARefusedEditLeavesEverythingAsItWas(t *testing.T) {
	call := api(t)
	create(t, call, workJSON)
	create(t, call, lunchJSON)
	snack := create(t, call, snackJSON)
	occurrence := "/v1/tasks/" + snack + "/occurrences/2030-01-08"
	require.Equal(t, http.StatusOK, call(http.MethodPut, occurrence, `{"start":"16:00","end":"16:30"}`).Code)

	onLunch := call(http.MethodPut, "/v1/tasks/"+snack, `{"title":"Lanche","area":"Alimentação","kind":"fixed",
		"weekdays":["monday"],"start":"12:30","end":"13:00"}`)

	assert.Equal(t, http.StatusConflict, onLunch.Code)
	var task struct {
		Start    string   `json:"start"`
		Weekdays []string `json:"weekdays"`
	}
	require.NoError(t, json.Unmarshal(call(http.MethodGet, "/v1/tasks/"+snack, "").Body.Bytes(), &task))
	assert.Equal(t, "15:00", task.Start)
	assert.Len(t, task.Weekdays, 5)
	assert.Equal(t, http.StatusNoContent, call(http.MethodDelete, occurrence, "").Code,
		"the Tuesday adjustment the edit would drop is still there")
}

func TestRecurringTasksDoNotTakeTime(t *testing.T) {
	call := api(t)
	create(t, call, workJSON)
	create(t, call, lunchJSON)

	rec := call(http.MethodPost, "/v1/tasks", recurringEnglish)

	assert.Equal(t, http.StatusCreated, rec.Code, "only its sessions take time")
}
