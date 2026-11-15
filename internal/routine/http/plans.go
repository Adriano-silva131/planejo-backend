package http

import (
	"time"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
)

type planBody struct {
	TaskID   string `json:"taskId" format:"uuid"`
	Title    string `json:"title"`
	Area     string `json:"area"`
	TaskKind string `json:"taskKind" enum:"recurring,one_off"`
	Source   string `json:"source" enum:"usual,session,checkin" doc:"usual: dia de costume previsto enquanto a semana ainda pede; session: sessão encaixada só por período; checkin: dia feito sem sessão"`
	Period   string `json:"period,omitempty" enum:"morning,afternoon,evening" doc:"Ausente quando a tarefa não tem período preferido"`
	Minutes  int    `json:"minutes,omitempty" doc:"Duração da sessão; ausente em pontual" example:"60"`
	Done     bool   `json:"done" doc:"A tarefa tem check-in neste dia; sempre true em checkin"`
}

type toPlaceBody struct {
	TaskID    string `json:"taskId" format:"uuid"`
	Title     string `json:"title"`
	Area      string `json:"area"`
	TaskKind  string `json:"taskKind" enum:"recurring,one_off"`
	Minutes   int    `json:"minutes,omitempty" doc:"Duração da sessão; ausente em pontual" example:"60"`
	Period    string `json:"period,omitempty" enum:"morning,afternoon,evening" doc:"Período preferido, se houver"`
	Deadline  string `json:"deadline,omitempty" format:"date" doc:"Só em one_off"`
	Remaining int    `json:"remaining" doc:"Quantas vezes ainda não têm dia nesta semana; sempre 1 em one_off" example:"3"`
	Missed    int    `json:"missed" doc:"Quantas das remaining são de dias que já passaram sem check-in; sempre 0 em one_off" example:"2"`
}

func toToPlaceBody(t domain.ToPlace) toPlaceBody {
	b := toPlaceBody{
		TaskID: t.TaskID.String(), Title: t.Title, Area: t.Area, TaskKind: string(t.Kind),
		Minutes: minutes(t.Duration), Period: string(t.Period), Remaining: t.Remaining, Missed: t.Missed,
	}
	if !t.Deadline.IsZero() {
		b.Deadline = t.Deadline.String()
	}
	return b
}

func minutes(d time.Duration) int {
	return int(d / time.Minute)
}
