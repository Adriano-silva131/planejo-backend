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
		Description: "Reserva um horário (start e end) ou só um período do dia (period) na semana, em qualquer dia, sem alterar a tarefa. A tarefa tem no máximo uma sessão por dia: encaixar de novo no mesmo dia substitui a sessão, mesmo de horário para período ou de período para horário. A sessão termina até a meia-noite. No máximo duas coisas podem acontecer ao mesmo tempo: o que criaria uma terceira é recusado com 409 schedule_overlap; sessões só por período não entram nessa conta.",
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusConflict},
	}, h.placeSession)
	huma.Register(api, huma.Operation{
		OperationID: "remove-session", Method: http.MethodDelete, Path: "/tasks/{id}/sessions/{date}", Tags: tags,
		Summary: "Tira a sessão da tarefa deste dia", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, h.removeSession)
}

// sessionFields asks for a session at start–end, or only in period.
type sessionFields struct {
	Start  string `json:"start,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Início, no fuso do usuário; obrigatório sem period" example:"19:00"`
	End    string `json:"end,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Fim; 00:00 é meia-noite. Obrigatório em pontual com start; em recorrente, sem ele a sessão dura o tempo da tarefa" example:"20:00"`
	Period string `json:"period,omitempty" enum:"morning,afternoon,evening" doc:"Só o período do dia, sem horário; não vai com start nem end" example:"evening"`
}

// check reports, per field, what is missing or does not fit the request.
func (f sessionFields) check() []error {
	if f.Period == "" {
		if f.Start == "" {
			return []error{fieldError("start", "Obrigatório; ou envie period.")}
		}
		return nil
	}
	var errs []error
	for _, field := range []struct{ name, value string }{{"start", f.Start}, {"end", f.End}} {
		if field.value != "" {
			errs = append(errs, fieldError(field.name, "Não se aplica com period: escolha um horário ou um período."))
		}
	}
	return errs
}

func (f sessionFields) placement() (domain.Placement, error) {
	start, err := calendar.ParseOptionalTimeOfDay(f.Start)
	if err != nil {
		return domain.Placement{}, err
	}
	end, err := calendar.ParseOptionalTimeOfDay(f.End)
	if err != nil {
		return domain.Placement{}, err
	}
	return domain.Placement{Start: start, End: end, Period: calendar.Period(f.Period)}, nil
}

type sessionBody struct {
	TaskID    string    `json:"taskId" format:"uuid"`
	Date      string    `json:"date" format:"date"`
	Start     string    `json:"start,omitempty" doc:"Ausente na sessão só por período" example:"19:00"`
	End       string    `json:"end,omitempty" doc:"24:00 quando termina à meia-noite; ausente na sessão só por período" example:"20:00"`
	Period    string    `json:"period,omitempty" enum:"morning,afternoon,evening" doc:"Presente só na sessão sem horário" example:"evening"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toSessionBody(s domain.Session) sessionBody {
	b := sessionBody{TaskID: s.TaskID.String(), Date: s.Date.String(), CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	if s.Timed() {
		b.Start, b.End = s.Start.String(), s.End.String()
	} else {
		b.Period = string(s.Period)
	}
	return b
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
	if errs := in.Body.check(); len(errs) > 0 {
		return nil, huma.Error422UnprocessableEntity("", errs...)
	}
	p, err := in.Body.placement()
	if err != nil {
		return nil, err
	}
	s, err := h.routine.PlaceSession(ctx, o.userID, o.taskID, o.date, p)
	if err != nil {
		return nil, err
	}
	return &placeSessionOutput{Body: toSessionBody(s)}, nil
}

func (h handler) removeSession(ctx context.Context, in *removeSessionInput) (*struct{}, error) {
	o, err := parseOccurrence(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	return nil, h.routine.RemoveSession(ctx, o.userID, o.taskID, o.date)
}
