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
		Description: "Os blocos de cada dia, montados a partir das tarefas fixas e dos ajustes desta semana.",
	}, h.week)
	huma.Register(api, huma.Operation{
		OperationID: "adjust-occurrence", Method: http.MethodPut, Path: "/tasks/{id}/occurrences/{date}", Tags: tags,
		Summary:     "Ajusta uma ocorrência de uma tarefa fixa só nesta semana",
		Description: "Tira a ocorrência ou muda o horário dela, sem alterar a tarefa. Ajustar de novo substitui o ajuste anterior.",
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, h.adjust)
	huma.Register(api, huma.Operation{
		OperationID: "reset-occurrence", Method: http.MethodDelete, Path: "/tasks/{id}/occurrences/{date}", Tags: tags,
		Summary: "Volta a ocorrência ao padrão da tarefa", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, h.reset)
}

type weekInput struct {
	Week string `query:"week" format:"date" required:"true" doc:"Qualquer dia da semana desejada" example:"2026-10-26"`
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
