package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/task/app"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

var tags = []string{"Tarefas"}

type handler struct {
	tasks *app.Tasks
}

// Register adds the task routes to the authenticated group.
func Register(api huma.API, tasks *app.Tasks) {
	h := handler{tasks: tasks}
	huma.Register(api, huma.Operation{
		OperationID: "create-task", Method: http.MethodPost, Path: "/tasks", Tags: tags,
		Summary: "Cria uma tarefa", DefaultStatus: http.StatusCreated,
		Errors: []int{http.StatusUnprocessableEntity},
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "list-tasks", Method: http.MethodGet, Path: "/tasks", Tags: tags,
		Summary: "Lista as tarefas, da mais nova para a mais antiga",
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-task", Method: http.MethodGet, Path: "/tasks/{id}", Tags: tags,
		Summary: "Retorna uma tarefa", Errors: []int{http.StatusNotFound},
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "update-task", Method: http.MethodPut, Path: "/tasks/{id}", Tags: tags,
		Summary: "Substitui os dados de uma tarefa",
		Errors:  []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/tasks/{id}", Tags: tags,
		Summary: "Apaga uma tarefa", DefaultStatus: http.StatusNoContent,
		Errors: []int{http.StatusNotFound},
	}, h.delete)
}

type taskOutput struct {
	Body taskBody
}

type createInput struct {
	Body taskFields
}

type idInput struct {
	ID string `path:"id" format:"uuid" doc:"Id da tarefa"`
}

type updateInput struct {
	ID   string `path:"id" format:"uuid" doc:"Id da tarefa"`
	Body taskFields
}

type listInput struct {
	Page    int `query:"page" minimum:"1" default:"1" doc:"Página, a partir de 1"`
	PerPage int `query:"perPage" minimum:"1" maximum:"100" default:"20" doc:"Tarefas por página"`
}

type pageMeta struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
}

type listOutput struct {
	Body struct {
		Data []taskBody `json:"data"`
		Meta pageMeta   `json:"meta"`
	}
}

func (h handler) create(ctx context.Context, in *createInput) (*taskOutput, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	d, err := parseDetails(in.Body)
	if err != nil {
		return nil, err
	}
	task, err := h.tasks.Create(ctx, userID, d)
	if err != nil {
		return nil, err
	}
	return &taskOutput{Body: toBody(task)}, nil
}

func (h handler) list(ctx context.Context, in *listInput) (*listOutput, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	tasks, total, err := h.tasks.List(ctx, userID, in.Page, in.PerPage)
	if err != nil {
		return nil, err
	}
	out := &listOutput{}
	out.Body.Data = make([]taskBody, 0, len(tasks))
	for _, t := range tasks {
		out.Body.Data = append(out.Body.Data, toBody(t))
	}
	out.Body.Meta = pageMeta{Total: total, Page: in.Page, PerPage: in.PerPage}
	return out, nil
}

func (h handler) get(ctx context.Context, in *idInput) (*taskOutput, error) {
	userID, id, err := target(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	task, err := h.tasks.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return &taskOutput{Body: toBody(task)}, nil
}

func (h handler) update(ctx context.Context, in *updateInput) (*taskOutput, error) {
	userID, id, err := target(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	d, err := parseDetails(in.Body)
	if err != nil {
		return nil, err
	}
	task, err := h.tasks.Update(ctx, userID, id, d)
	if err != nil {
		return nil, err
	}
	return &taskOutput{Body: toBody(task)}, nil
}

func (h handler) delete(ctx context.Context, in *idInput) (*struct{}, error) {
	userID, id, err := target(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return nil, h.tasks.Delete(ctx, userID, id)
}

func parseDetails(f taskFields) (domain.Details, error) {
	if errs := f.checkKind(); len(errs) > 0 {
		return domain.Details{}, huma.Error422UnprocessableEntity("", errs...)
	}
	return f.details()
}

func currentUser(ctx context.Context) (uuid.UUID, error) {
	user, ok := auth.UserFrom(ctx)
	if !ok {
		return uuid.Nil, huma.Error401Unauthorized("")
	}
	return user.ID, nil
}

// target returns the user and the task id from the path.
func target(ctx context.Context, rawID string) (userID, id uuid.UUID, err error) {
	userID, err = currentUser(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	id, err = uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, uuid.Nil, domain.ErrTaskNotFound
	}
	return userID, id, nil
}
