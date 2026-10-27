package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/app"
	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

var tags = []string{"Progresso"}

type handler struct {
	progress *app.Progress
}

// Register adds the progress routes to the authenticated group.
func Register(api huma.API, progress *app.Progress) {
	h := handler{progress: progress}
	huma.Register(api, huma.Operation{
		OperationID: "check-in", Method: http.MethodPut, Path: "/tasks/{id}/checkins/{date}", Tags: tags,
		Summary:     "Marca a tarefa como feita no dia",
		Description: "Recorrentes: sem corpo. Pontuais: minutos do dia e, se for o caso, a conclusão. Registrar de novo o mesmo dia substitui o anterior.",
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, h.checkIn)
	huma.Register(api, huma.Operation{
		OperationID: "undo-check-in", Method: http.MethodDelete, Path: "/tasks/{id}/checkins/{date}", Tags: tags,
		Summary: "Desfaz o registro do dia", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, h.undo)
	huma.Register(api, huma.Operation{
		OperationID: "list-check-ins", Method: http.MethodGet, Path: "/tasks/{id}/checkins", Tags: tags,
		Summary: "Lista os registros da tarefa, do mais antigo para o mais novo",
		Errors:  []int{http.StatusNotFound},
	}, h.history)
	huma.Register(api, huma.Operation{
		OperationID: "week-progress", Method: http.MethodGet, Path: "/progress", Tags: tags,
		Summary: "Resumo do progresso da semana",
	}, h.week)
}

type dayInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa"`
	Date string `path:"date" format:"date" doc:"Dia em que a tarefa foi feita" example:"2026-10-27"`
}

type checkInInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa"`
	Date string `path:"date" format:"date" doc:"Dia em que a tarefa foi feita" example:"2026-10-27"`
	Body *checkInFields
}

type checkInOutput struct {
	Body checkInBody
}

type historyInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa"`
	From string `query:"from" format:"date" doc:"Primeiro dia, opcional"`
	To   string `query:"to" format:"date" doc:"Último dia, opcional"`
}

type historyOutput struct {
	Body struct {
		Data []checkInBody `json:"data"`
	}
}

type weekInput struct {
	Week string `query:"week" format:"date" required:"true" doc:"Qualquer dia da semana desejada" example:"2026-10-26"`
}

type weekOutput struct {
	Body weekBody
}

func (h handler) checkIn(ctx context.Context, in *checkInInput) (*checkInOutput, error) {
	d, err := parseDay(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	e := domain.Entry{Date: d.date}
	if in.Body != nil {
		e.Minutes, e.Completed = in.Body.Minutes, in.Body.Completed
	}
	c, err := h.progress.CheckIn(ctx, d.userID, d.taskID, e)
	if err != nil {
		return nil, err
	}
	return &checkInOutput{Body: toCheckInBody(c)}, nil
}

func (h handler) undo(ctx context.Context, in *dayInput) (*struct{}, error) {
	d, err := parseDay(ctx, in.ID, in.Date)
	if err != nil {
		return nil, err
	}
	return nil, h.progress.Undo(ctx, d.userID, d.taskID, d.date)
}

func (h handler) history(ctx context.Context, in *historyInput) (*historyOutput, error) {
	userID, taskID, err := userAndTask(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	f := domain.Filter{TaskID: taskID}
	if f.From, err = parseOptionalDate(in.From); err != nil {
		return nil, err
	}
	if f.To, err = parseOptionalDate(in.To); err != nil {
		return nil, err
	}
	checkIns, err := h.progress.History(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	out := &historyOutput{}
	out.Body.Data = make([]checkInBody, 0, len(checkIns))
	for _, c := range checkIns {
		out.Body.Data = append(out.Body.Data, toCheckInBody(c))
	}
	return out, nil
}

func (h handler) week(ctx context.Context, in *weekInput) (*weekOutput, error) {
	user, ok := auth.UserFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("")
	}
	day, err := calendar.ParseDate(in.Week)
	if err != nil {
		return nil, err
	}
	s, err := h.progress.Week(ctx, user.ID, day)
	if err != nil {
		return nil, err
	}
	return &weekOutput{Body: toWeekBody(s)}, nil
}

// day is the user, task and date a check-in route acts on.
type day struct {
	userID uuid.UUID
	taskID uuid.UUID
	date   calendar.Date
}

func parseDay(ctx context.Context, rawID, rawDate string) (day, error) {
	userID, taskID, err := userAndTask(ctx, rawID)
	if err != nil {
		return day{}, err
	}
	date, err := calendar.ParseDate(rawDate)
	if err != nil {
		return day{}, err
	}
	return day{userID: userID, taskID: taskID, date: date}, nil
}

// userAndTask returns the authenticated user and the task id from the path.
// The id format is checked by huma, so a parse failure means no such task.
func userAndTask(ctx context.Context, rawID string) (userID, taskID uuid.UUID, err error) {
	user, ok := auth.UserFrom(ctx)
	if !ok {
		return uuid.Nil, uuid.Nil, huma.Error401Unauthorized("")
	}
	taskID, err = uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, uuid.Nil, huma.Error404NotFound("")
	}
	return user.ID, taskID, nil
}
