package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/app"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

var tags = []string{"Rotina"}

type handler struct {
	routine *app.Routine
}

// Register adds the routine routes to the authenticated group.
func Register(api huma.API, routine *app.Routine) {
	h := handler{routine: routine}
	huma.Register(api, huma.Operation{
		OperationID: "week-routine", Method: http.MethodGet, Path: "/routine", Tags: tags,
		Summary:     "Rotina da semana",
		Description: "Os blocos de cada dia: as tarefas fixas, com os ajustes desta semana, e as sessões com horário que o usuário encaixou para recorrentes e pontuais. Em plans, o que está no dia sem horário: sessões só por período, check-ins sem sessão e os dias de costume previstos de hoje em diante. Em toPlace, o que ainda não tem dia na semana. Hoje é o dia no fuso do cabeçalho Time-Zone, ou em UTC sem ele.",
	}, h.week)
	huma.Register(api, huma.Operation{
		OperationID: "adjust-occurrence", Method: http.MethodPut, Path: "/tasks/{id}/occurrences/{date}", Tags: tags,
		Summary:     "Ajusta uma ocorrência de uma tarefa fixa só neste dia",
		Description: "Tira a ocorrência ou muda o horário dela, sem alterar a tarefa. Ajustar de novo substitui o ajuste anterior. Recorrentes e pontuais não têm ocorrências: use as sessões. No máximo duas coisas podem acontecer ao mesmo tempo: o que criaria uma terceira é recusado com 409 schedule_overlap.",
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusConflict},
	}, h.adjust)
	huma.Register(api, huma.Operation{
		OperationID: "reset-occurrence", Method: http.MethodDelete, Path: "/tasks/{id}/occurrences/{date}", Tags: tags,
		Summary: "Volta a ocorrência ao padrão da tarefa", DefaultStatus: http.StatusNoContent,
		Description: "No máximo duas coisas podem acontecer ao mesmo tempo: o que criaria uma terceira é recusado com 409 schedule_overlap.",
		Errors:      []int{http.StatusNotFound, http.StatusConflict},
	}, h.reset)
	registerSessions(api, h)
}

type weekInput struct {
	Week     string `query:"week" format:"date" required:"true" doc:"Qualquer dia da semana desejada" example:"2026-10-26"`
	TimeZone string `header:"Time-Zone" doc:"Fuso do aparelho, no formato IANA; define qual é o dia de hoje. Sem ele, UTC" example:"America/Sao_Paulo"`
}

type weekOutput struct {
	Body routineWeekBody
}

type adjustInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa fixa"`
	Date string `path:"date" format:"date" doc:"Dia em que a ocorrência começa" example:"2026-10-26"`
	Body changeFields
}

type adjustOutput struct {
	Body adjustmentBody
}

type resetInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa fixa"`
	Date string `path:"date" format:"date" doc:"Dia em que a ocorrência começa" example:"2026-10-26"`
}

func (h handler) week(ctx context.Context, in *weekInput) (*weekOutput, error) {
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	day, err := calendar.ParseDate(in.Week)
	if err != nil {
		return nil, err
	}
	// The middleware already stored a valid zone in ctx; an invalid one is refused here.
	if in.TimeZone != "" {
		if _, err := httpx.ParseTimeZone(in.TimeZone); err != nil {
			return nil, err
		}
	}
	w, err := h.routine.Week(ctx, userID, day)
	if err != nil {
		return nil, err
	}
	return &weekOutput{Body: toWeekBody(w)}, nil
}

func (h handler) adjust(ctx context.Context, in *adjustInput) (*adjustOutput, error) {
	o, err := parseOccurrence(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	if errs := in.Body.check(); len(errs) > 0 {
		return nil, huma.Error422UnprocessableEntity("", errs...)
	}
	c, err := in.Body.change()
	if err != nil {
		return nil, err
	}
	a, err := h.routine.Adjust(ctx, o.userID, o.taskID, o.date, c)
	if err != nil {
		return nil, err
	}
	return &adjustOutput{Body: toAdjustmentBody(a)}, nil
}

func (h handler) reset(ctx context.Context, in *resetInput) (*struct{}, error) {
	o, err := parseOccurrence(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	return nil, h.routine.Reset(ctx, o.userID, o.taskID, o.date)
}

// occurrence is the user, task and day an occurrence route acts on.
type occurrence struct {
	userID uuid.UUID
	taskID uuid.UUID
	date   calendar.Date
}

// parseOccurrence reads the path, whose formats huma already checked: an id
// that does not parse means no such task.
func parseOccurrence(ctx context.Context, rawID, rawDate string) (occurrence, error) {
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return occurrence{}, err
	}
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return occurrence{}, huma.Error404NotFound("")
	}
	date, err := calendar.ParseDate(rawDate)
	if err != nil {
		return occurrence{}, err
	}
	return occurrence{userID: userID, taskID: taskID, date: date}, nil
}
