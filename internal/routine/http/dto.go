package http

import (
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// changeFields asks to skip the occurrence, or to move it to start–end.
type changeFields struct {
	Skipped bool   `json:"skipped,omitempty" doc:"Tira a ocorrência desta semana; sem start e end"`
	Start   string `json:"start,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Novo início, no fuso do usuário" example:"09:00"`
	End     string `json:"end,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"Novo fim, obrigatório ao mudar o horário; antes do início, termina no dia seguinte" example:"16:00"`
}

// check reports, per field, what is missing or does not fit the request.
func (f changeFields) check() []error {
	var errs []error
	if f.Skipped {
		for name, value := range map[string]string{"start": f.Start, "end": f.End} {
			if value != "" {
				errs = append(errs, fieldError(name, "Não se aplica ao tirar a ocorrência."))
			}
		}
		return errs
	}
	if f.Start == "" {
		errs = append(errs, fieldError("start", "Obrigatório para mudar o horário; ou envie skipped."))
	}
	if f.End == "" {
		errs = append(errs, fieldError("end", "Obrigatório para mudar o horário; ou envie skipped."))
	}
	return errs
}

func fieldError(name, message string) error {
	return &huma.ErrorDetail{Location: "body." + name, Message: message}
}

func (f changeFields) change() (domain.Change, error) {
	if f.Skipped {
		return domain.Change{Skip: true}, nil
	}
	start, err := calendar.ParseTimeOfDay(f.Start)
	if err != nil {
		return domain.Change{}, err
	}
	end, err := calendar.ParseTimeOfDay(f.End)
	if err != nil {
		return domain.Change{}, err
	}
	return domain.Change{Start: start, End: end}, nil
}

type adjustmentBody struct {
	TaskID    string    `json:"taskId" format:"uuid"`
	Date      string    `json:"date" format:"date" doc:"Dia em que a ocorrência começa"`
	Skipped   bool      `json:"skipped"`
	Start     string    `json:"start,omitempty" doc:"Ausente quando skipped"`
	End       string    `json:"end,omitempty" doc:"Ausente quando skipped"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toAdjustmentBody(a domain.Adjustment) adjustmentBody {
	b := adjustmentBody{
		TaskID:    a.TaskID.String(),
		Date:      a.Date.String(),
		Skipped:   a.Skipped,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
	if !a.Skipped {
		b.Start, b.End = a.Start.String(), a.End.String()
	}
	return b
}

type blockBody struct {
	TaskID   string `json:"taskId" format:"uuid"`
	Title    string `json:"title"`
	Area     string `json:"area"`
	TaskKind string `json:"taskKind" enum:"fixed,recurring,one_off" doc:"recurring e one_off: sessão que o usuário encaixou neste dia"`
	Start    string `json:"start" example:"09:00"`
	End      string `json:"end" doc:"24:00 quando continua no dia seguinte ou termina à meia-noite" example:"18:00"`
	Adjusted bool   `json:"adjusted" doc:"fixed: o horário desta semana difere do padrão da tarefa"`
	Done     bool   `json:"done" doc:"recurring e one_off: a tarefa tem check-in neste dia; fixed sempre false"`

	ContinuesFromPreviousDay bool `json:"continuesFromPreviousDay" doc:"Começou no dia anterior"`
	ContinuesNextDay         bool `json:"continuesNextDay" doc:"Termina no dia seguinte"`
}

type dayBody struct {
	Date           string        `json:"date" format:"date"`
	Weekday        string        `json:"weekday" example:"monday"`
	Blocks         []blockBody   `json:"blocks" doc:"Em ordem de início"`
	PeriodSessions []sessionBody `json:"periodSessions" doc:"Sessões só por período, sem ocupar horário"`
	Plans          []planBody    `json:"plans" doc:"Recorrentes e pontuais no dia sem horário, em ordem de período (manhã, tarde, noite, sem período) e título"`
}

type routineWeekBody struct {
	Start   string        `json:"start" format:"date" doc:"Segunda-feira da semana"`
	End     string        `json:"end" format:"date" doc:"Domingo da semana"`
	Days    []dayBody     `json:"days" doc:"Os 7 dias, de segunda a domingo"`
	ToPlace []toPlaceBody `json:"toPlace" doc:"O que ainda não tem dia nesta semana"`
}

func toWeekBody(w domain.Week) routineWeekBody {
	b := routineWeekBody{
		Start: w.Start.String(), End: w.End.String(),
		Days: make([]dayBody, 0, len(w.Days)), ToPlace: make([]toPlaceBody, 0, len(w.ToPlace)),
	}
	for _, d := range w.Days {
		b.Days = append(b.Days, toDayBody(d))
	}
	for _, t := range w.ToPlace {
		b.ToPlace = append(b.ToPlace, toToPlaceBody(t))
	}
	return b
}

func toDayBody(d domain.Day) dayBody {
	day := dayBody{
		Date:           d.Date.String(),
		Weekday:        calendar.WeekdayName(d.Date.Weekday()),
		Blocks:         make([]blockBody, 0, len(d.Blocks)),
		Plans:          make([]planBody, 0, len(d.Plans)),
		PeriodSessions: make([]sessionBody, 0, len(d.PeriodSessions)),
	}
	for _, bl := range d.Blocks {
		day.Blocks = append(day.Blocks, blockBody{
			TaskID:                   bl.TaskID.String(),
			Title:                    bl.Title,
			Area:                     bl.Area,
			TaskKind:                 string(bl.Kind),
			Start:                    bl.Start.String(),
			End:                      bl.End.String(),
			Adjusted:                 bl.Adjusted,
			Done:                     bl.Done,
			ContinuesFromPreviousDay: bl.ContinuesFromPreviousDay,
			ContinuesNextDay:         bl.ContinuesNextDay,
		})
	}
	for _, s := range d.PeriodSessions {
		day.PeriodSessions = append(day.PeriodSessions, toSessionBody(s))
	}
	for _, p := range d.Plans {
		day.Plans = append(day.Plans, planBody{
			TaskID: p.TaskID.String(), Title: p.Title, Area: p.Area, TaskKind: string(p.Kind),
			Source: string(p.Source), Period: string(p.Period), Minutes: minutes(p.Duration), Done: p.Done,
		})
	}
	return day
}
