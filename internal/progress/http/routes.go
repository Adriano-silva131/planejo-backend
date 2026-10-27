package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
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
	huma.Register(api, huma.Operation{
		OperationID: "habits", Method: http.MethodGet, Path: "/progress/habits", Tags: tags,
		Summary:     "Hábitos: as tarefas recorrentes nas últimas semanas",
		Description: "Para cada recorrente, a grade dos dias feitos e a sequência de semanas cumpridas.",
	}, h.habits)
}

type dayInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa"`
	Date string `path:"date" format:"date" doc:"Dia em que a tarefa foi feita" example:"2026-10-27"`
}

type checkInInput struct {
	ID       string `path:"id" format:"uuid" doc:"Id da tarefa"`
	Date     string `path:"date" format:"date" doc:"Dia em que a tarefa foi feita" example:"2026-10-27"`
	TimeZone string `header:"Time-Zone" required:"true" doc:"Fuso do aparelho, no formato IANA; define qual é o dia de hoje" example:"America/Sao_Paulo"`
	Body     *checkInFields
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
	loc, err := parseTimeZone(in.TimeZone)
	if err != nil {
		return nil, err
	}
	e := domain.Entry{Date: d.date}
	if in.Body != nil {
		e.Minutes, e.Completed = in.Body.Minutes, in.Body.Completed
	}
	c, err := h.progress.CheckIn(ctx, d.userID, d.taskID, e, loc)
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
	if f.From, err = calendar.ParseOptionalDate(in.From); err != nil {
		return nil, err
	}
	if f.To, err = calendar.ParseOptionalDate(in.To); err != nil {
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
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	day, err := calendar.ParseDate(in.Week)
	if err != nil {
		return nil, err
	}
	s, err := h.progress.Week(ctx, userID, day)
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

type habitsInput struct {
	Week  string `query:"week" format:"date" required:"true" doc:"Um dia da semana de referência, normalmente hoje" example:"2026-10-27"`
	Weeks int    `query:"weeks" minimum:"1" maximum:"26" default:"8" doc:"Quantas semanas mostrar na grade"`
}

type habitsOutput struct {
	Body struct {
		WeekStart string      `json:"weekStart" format:"date" doc:"Segunda-feira da semana de referência"`
		Habits    []habitBody `json:"habits"`
	}
}

func (h handler) habits(ctx context.Context, in *habitsInput) (*habitsOutput, error) {
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	day, err := calendar.ParseDate(in.Week)
	if err != nil {
		return nil, err
	}
	habits, err := h.progress.Habits(ctx, userID, day, in.Weeks)
	if err != nil {
		return nil, err
	}
	out := &habitsOutput{}
	out.Body.WeekStart = day.StartOfWeek().String()
	out.Body.Habits = make([]habitBody, 0, len(habits))
	for _, habit := range habits {
		out.Body.Habits = append(out.Body.Habits, toHabitBody(habit))
	}
	return out, nil
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

// parseTimeZone reads an IANA zone name such as "America/Sao_Paulo". "Local"
// is refused: it would be the server's zone, not the user's.
func parseTimeZone(name string) (*time.Location, error) {
	loc, err := time.LoadLocation(name)
	if err != nil || name == "" || name == "Local" {
		return nil, huma.Error422UnprocessableEntity("", &huma.ErrorDetail{
			Location: "header.Time-Zone",
			Message:  "Fuso horário inválido; use um nome como America/Sao_Paulo.",
		})
	}
	return loc, nil
}

// userAndTask returns the authenticated user and the task id from the path.
// The id format is checked by huma, so a parse failure means no such task.
func userAndTask(ctx context.Context, rawID string) (userID, taskID uuid.UUID, err error) {
	if userID, err = httpx.CurrentUser(ctx); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if taskID, err = uuid.Parse(rawID); err != nil {
		return uuid.Nil, uuid.Nil, huma.Error404NotFound("")
	}
	return userID, taskID, nil
}
