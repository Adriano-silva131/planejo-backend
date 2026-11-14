package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

func registerSessions(api huma.API, h handler) {
	huma.Register(api, huma.Operation{
		OperationID: "place-session", Method: http.MethodPut, Path: "/tasks/{id}/sessions/{date}", Tags: tags,
		Summary:     "Encaixa uma sessão de uma tarefa recorrente ou pontual num dia",
		Description: "Reserva o horário na semana, em qualquer dia, sem alterar a tarefa. A tarefa tem no máximo uma sessão por dia: encaixar de novo no mesmo dia muda o horário. A sessão termina até a meia-noite. No máximo duas coisas podem acontecer ao mesmo tempo: o que criaria uma terceira é recusado com 409 schedule_overlap.",
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusConflict},
	}, h.placeSession)
	huma.Register(api, huma.Operation{
		OperationID: "remove-session", Method: http.MethodDelete, Path: "/tasks/{id}/sessions/{date}", Tags: tags,
		Summary: "Tira a sessão da tarefa deste dia", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, h.removeSession)
}

type sessionFields struct {
	Start string `json:"start" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Início, no fuso do usuário" example:"19:00"`
	End   string `json:"end,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Fim; 00:00 é meia-noite. Obrigatório em pontual; em recorrente, sem ele a sessão dura o tempo da tarefa" example:"20:00"`
}

func (f sessionFields) placement() (domain.Placement, error) {
	start, err := calendar.ParseTimeOfDay(f.Start)
	if err != nil {
		return domain.Placement{}, err
	}
	end, err := calendar.ParseOptionalTimeOfDay(f.End)
	if err != nil {
		return domain.Placement{}, err
	}
	return domain.Placement{Start: start, End: end}, nil
}

type sessionBody struct {
	TaskID    string    `json:"taskId" format:"uuid"`
	Date      string    `json:"date" format:"date"`
	Start     string    `json:"start" example:"19:00"`
	End       string    `json:"end" doc:"24:00 quando termina à meia-noite" example:"20:00"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type placeSessionInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa recorrente ou pontual"`
	Date string `path:"date" format:"date" doc:"Dia da sessão" example:"2026-10-28"`
	Body sessionFields
}

type placeSessionOutput struct {
	Body sessionBody
}

type removeSessionInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa recorrente ou pontual"`
	Date string `path:"date" format:"date" doc:"Dia da sessão" example:"2026-10-28"`
}

func (h handler) placeSession(ctx context.Context, in *placeSessionInput) (*placeSessionOutput, error) {
	o, err := parseOccurrence(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	p, err := in.Body.placement()
	if err != nil {
		return nil, err
	}
	s, err := h.routine.PlaceSession(ctx, o.userID, o.taskID, o.date, p)
	if err != nil {
		return nil, err
	}
	return &placeSessionOutput{Body: sessionBody{
		TaskID: s.TaskID.String(), Date: s.Date.String(), Start: s.Start.String(), End: s.End.String(),
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}}, nil
}

func (h handler) removeSession(ctx context.Context, in *removeSessionInput) (*struct{}, error) {
	o, err := parseOccurrence(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	return nil, h.routine.RemoveSession(ctx, o.userID, o.taskID, o.date)
}
