package http

import (
	"time"

	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type checkInFields struct {
	Minutes   int  `json:"minutes,omitempty" minimum:"0" maximum:"1440" doc:"Só em tarefa pontual: minutos dedicados no dia" example:"90"`
	Completed bool `json:"completed,omitempty" doc:"Só em tarefa pontual: concluída neste dia"`
}

// checkInBody always carries minutes and completed, even when zero.
type checkInBody struct {
	TaskID    string    `json:"taskId" format:"uuid"`
	Date      string    `json:"date" format:"date"`
	Minutes   int       `json:"minutes" doc:"Minutos dedicados no dia; 0 em tarefa recorrente"`
	Completed bool      `json:"completed" doc:"Concluída neste dia; só em tarefa pontual"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toCheckInBody(c domain.CheckIn) checkInBody {
	return checkInBody{
		TaskID:    c.TaskID.String(),
		Date:      c.Date.String(),
		Minutes:   c.Minutes,
		Completed: c.Completed,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type recurringBody struct {
	TaskID   string   `json:"taskId" format:"uuid"`
	Title    string   `json:"title"`
	Expected int      `json:"expected" doc:"Vezes por semana em que a tarefa está marcada"`
	Done     int      `json:"done" doc:"Vezes registradas na semana, em qualquer dia"`
	Days     []string `json:"days" doc:"Dias registrados"`
}

type oneOffBody struct {
	TaskID       string `json:"taskId" format:"uuid"`
	Title        string `json:"title"`
	TotalMinutes int    `json:"totalMinutes" doc:"Duração total planejada"`
	Deadline     string `json:"deadline" format:"date"`
	WeekMinutes  int    `json:"weekMinutes" doc:"Minutos registrados na semana"`
	DoneMinutes  int    `json:"doneMinutes" doc:"Minutos registrados até o fim da semana"`
	CompletedOn  string `json:"completedOn,omitempty" format:"date" doc:"Dia da conclusão; ausente enquanto aberta"`
}

type weekBody struct {
	Start     string          `json:"start" format:"date" doc:"Segunda-feira da semana"`
	End       string          `json:"end" format:"date" doc:"Domingo da semana"`
	Recurring []recurringBody `json:"recurring"`
	OneOff    []oneOffBody    `json:"oneOff"`
}

func toWeekBody(s domain.WeekSummary) weekBody {
	b := weekBody{
		Start:     s.Start.String(),
		End:       s.End.String(),
		Recurring: make([]recurringBody, 0, len(s.Recurring)),
		OneOff:    make([]oneOffBody, 0, len(s.OneOff)),
	}
	for _, p := range s.Recurring {
		b.Recurring = append(b.Recurring, recurringBody{
			TaskID:   p.Task.ID.String(),
			Title:    p.Task.Title,
			Expected: p.Expected,
			Done:     len(p.Done),
			Days:     dateStrings(p.Done),
		})
	}
	for _, p := range s.OneOff {
		b.OneOff = append(b.OneOff, oneOffBody{
			TaskID:       p.Task.ID.String(),
			Title:        p.Task.Title,
			TotalMinutes: p.Task.TotalMinutes,
			Deadline:     p.Task.Deadline.String(),
			WeekMinutes:  p.WeekMinutes,
			DoneMinutes:  p.DoneMinutes,
			CompletedOn:  p.CompletedOn.String(),
		})
	}
	return b
}

func dateStrings(dates []calendar.Date) []string {
	out := make([]string, 0, len(dates))
	for _, d := range dates {
		out = append(out, d.String())
	}
	return out
}
