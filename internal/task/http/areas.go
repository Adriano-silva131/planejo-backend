package http

import (
	"context"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
)

type areaBody struct {
	Area  string `json:"area" example:"Saúde"`
	Tasks int    `json:"tasks" doc:"Tarefas com esta área" example:"3"`
	Color string `json:"color,omitempty" doc:"Cor escolhida pelo usuário, #rrggbb; ausente enquanto ele não escolher" example:"#e08a5c"`
}

type areasOutput struct {
	Body struct {
		Data []areaBody `json:"data"`
	}
}

type areaColorBody struct {
	Area  string `json:"area" minLength:"1" maxLength:"50" example:"Saúde"`
	Color string `json:"color" pattern:"^#[0-9a-fA-F]{6}$" patternDescription:"#rrggbb" doc:"Cor base; o app deriva dela o fundo, a borda e o texto de cada tema" example:"#e08a5c"`
}

type areaColorInput struct {
	Body areaColorBody
}

type areaColorOutput struct {
	Body areaColorBody
}

func (h handler) areas(ctx context.Context, _ *struct{}) (*areasOutput, error) {
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	areas, err := h.tasks.Areas(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &areasOutput{}
	out.Body.Data = make([]areaBody, 0, len(areas))
	for _, a := range areas {
		out.Body.Data = append(out.Body.Data, areaBody{Area: a.Area, Tasks: a.Tasks, Color: a.Color})
	}
	return out, nil
}

func (h handler) setAreaColor(ctx context.Context, in *areaColorInput) (*areaColorOutput, error) {
	userID, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.tasks.SetAreaColor(ctx, userID, in.Body.Area, in.Body.Color)
	if err != nil {
		return nil, err
	}
	return &areaColorOutput{Body: areaColorBody{Area: c.Area, Color: c.Color}}, nil
}
