package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
)

func TestPlaceSessionOnlyInAPeriod(t *testing.T) {
	srv := newServer(t)
	srv.workAndLunch()
	gymID := srv.addPlannable(userA, gym())
	reportID := srv.addPlannable(userA, domain.PlannableTask{Title: "Relatório", Area: "Trabalho", Kind: domain.OneOffKind})
	path := "/v1/tasks/" + gymID + "/sessions/2026-10-26"

	evening := srv.do(http.MethodPut, path, "token-a", `{"period":"evening"}`)
	report := srv.do(http.MethodPut, "/v1/tasks/"+reportID+"/sessions/2026-10-26", "token-a", `{"period":"afternoon"}`)

	require.Equal(t, http.StatusOK, evening.Code, evening.Body.String())
	require.Equal(t, http.StatusOK, report.Code, report.Body.String(), "takes no layer, nor needs an end")
	body := decode(t, evening)
	assert.Equal(t, "evening", body["period"])
	assert.NotContains(t, body, "start")
	assert.NotContains(t, body, "end")
	week := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-26", "token-a", ""))
	assert.Equal(t, []string{"Trabalho 09:00-18:00", "Almoço 12:00-13:00"}, blocks(t, week, 0), "no block without a time")
}

func TestPlaceSessionSwitchesBetweenTimeAndPeriod(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addPlannable(userA, gym()) + "/sessions/2026-10-28"

	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", `{"start":"07:00"}`).Code)
	require.Equal(t, http.StatusOK, srv.do(http.MethodPut, path, "token-a", `{"period":"morning"}`).Code)
	week := decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-28", "token-a", ""))
	assert.Empty(t, blocks(t, week, 2), "the period replaced the time")

	timed := srv.do(http.MethodPut, path, "token-a", `{"start":"19:00"}`)
	require.Equal(t, http.StatusOK, timed.Code)
	assert.NotContains(t, decode(t, timed), "period")
	week = decode(t, srv.do(http.MethodGet, "/v1/routine?week=2026-10-28", "token-a", ""))
	assert.Equal(t, []string{"Academia 19:00-20:00"}, blocks(t, week, 2))
}

func TestPlaceSessionReportsFields(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addPlannable(userA, gym()) + "/sessions/2026-10-28"
	withPeriod := "Não se aplica com period: escolha um horário ou um período."

	cases := map[string]struct {
		body   string
		fields map[string]any
	}{
		"period and time": {`{"period":"evening","start":"19:00","end":"20:00"}`, map[string]any{
			"start": []any{withPeriod}, "end": []any{withPeriod},
		}},
		"neither": {`{"end":"20:00"}`, map[string]any{"start": []any{"Obrigatório; ou envie period."}}},
		"empty":   {`{}`, map[string]any{"start": []any{"Obrigatório; ou envie period."}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := srv.do(http.MethodPut, path, "token-a", tc.body)

			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			assert.Equal(t, tc.fields, errorOf(t, rec)["fields"])
		})
	}
}

func TestPlaceSessionRejectsAnUnknownPeriod(t *testing.T) {
	srv := newServer(t)
	path := "/v1/tasks/" + srv.addPlannable(userA, gym()) + "/sessions/2026-10-28"

	rec := srv.do(http.MethodPut, path, "token-a", `{"period":"dawn"}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, errorOf(t, rec)["fields"], "period")
}
