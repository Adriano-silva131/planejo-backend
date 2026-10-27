package http

import (
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

// taskFields is the task as the client sends it. Schedule fields depend on
// kind; fieldsByKind says which ones each kind requires and accepts.
type taskFields struct {
	Title string `json:"title" minLength:"1" maxLength:"120" doc:"Título da tarefa" example:"Academia"`
	Area  string `json:"area" minLength:"1" maxLength:"50" doc:"Tipo da tarefa, escolhido pelo usuário, ex.: Saúde, Trabalho, Estudos; GET /tasks/areas lista os já usados" example:"Saúde"`
	Kind  string `json:"kind" enum:"fixed,recurring,one_off" doc:"fixed: hora marcada, o planejador só contorna; recurring: repete nos dias escolhidos; one_off: uma vez, até o prazo" example:"recurring"`

	Priority string   `json:"priority,omitempty" enum:"high,medium,low" doc:"recurring e one_off" example:"high"`
	Weekdays []string `json:"weekdays,omitempty" enum:"sunday,monday,tuesday,wednesday,thursday,friday,saturday" uniqueItems:"true" doc:"recurring, ou fixed semanal"`
	Date     string   `json:"date,omitempty" format:"date" doc:"fixed num dia específico, no lugar de weekdays"`
	Start    string   `json:"start,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"fixed: início, no fuso do usuário" example:"09:00"`
	End      string   `json:"end,omitempty" pattern:"^([01][0-9]|2[0-3]):[0-5][0-9]$" patternDescription:"HH:MM" doc:"fixed: fim, no fuso do usuário; antes do início, termina no dia seguinte" example:"18:00"`

	DurationMinutes int    `json:"durationMinutes,omitempty" minimum:"5" maximum:"6000" doc:"recurring: por sessão; one_off: total" example:"60"`
	Deadline        string `json:"deadline,omitempty" format:"date" doc:"one_off: último dia para concluir"`
	Period          string `json:"period,omitempty" enum:"morning,afternoon,evening" doc:"recurring e one_off: período preferido, opcional" example:"morning"`
}

type taskBody struct {
	ID string `json:"id" format:"uuid"`
	taskFields
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type kindFields struct {
	required []string
	optional []string
}

var fieldsByKind = map[domain.Kind]kindFields{
	domain.Fixed:     {required: []string{"start", "end"}, optional: []string{"weekdays", "date"}},
	domain.Recurring: {required: []string{"priority", "weekdays", "durationMinutes"}, optional: []string{"period"}},
	domain.OneOff:    {required: []string{"priority", "durationMinutes", "deadline"}, optional: []string{"period"}},
}

var scheduleFields = []string{"priority", "weekdays", "date", "start", "end", "durationMinutes", "deadline", "period"}

func (f taskFields) present() map[string]bool {
	return map[string]bool{
		"priority":        f.Priority != "",
		"weekdays":        len(f.Weekdays) > 0,
		"date":            f.Date != "",
		"start":           f.Start != "",
		"end":             f.End != "",
		"durationMinutes": f.DurationMinutes != 0,
		"deadline":        f.Deadline != "",
		"period":          f.Period != "",
	}
}

// checkKind reports, per field, what the kind is missing or does not take.
func (f taskFields) checkKind() []error {
	spec := fieldsByKind[domain.Kind(f.Kind)]
	present := f.present()
	var errs []error
	for _, name := range scheduleFields {
		required := slices.Contains(spec.required, name)
		switch {
		case required && !present[name]:
			errs = append(errs, fieldError(name, "Obrigatório para este tipo de tarefa."))
		case !required && present[name] && !slices.Contains(spec.optional, name):
			errs = append(errs, fieldError(name, "Não se aplica a este tipo de tarefa."))
		}
	}
	return errs
}

func fieldError(name, message string) error {
	return &huma.ErrorDetail{Location: "body." + name, Message: message}
}

func (f taskFields) details() (domain.Details, error) {
	schedule, err := f.schedule()
	if err != nil {
		return domain.Details{}, err
	}
	return domain.Details{
		Title:    f.Title,
		Area:     f.Area,
		Priority: domain.Priority(f.Priority),
		Schedule: schedule,
	}, nil
}

func (f taskFields) schedule() (domain.Schedule, error) {
	duration := time.Duration(f.DurationMinutes) * time.Minute
	switch domain.Kind(f.Kind) {
	case domain.Fixed:
		return f.fixedSchedule()
	case domain.Recurring:
		days, err := calendar.ParseWeekdayNames(f.Weekdays)
		if err != nil {
			return nil, err
		}
		return domain.RecurringSchedule{Weekdays: days, Duration: duration, Period: domain.Period(f.Period)}, nil
	case domain.OneOff:
		deadline, err := calendar.ParseOptionalDate(f.Deadline)
		if err != nil {
			return nil, err
		}
		return domain.OneOffSchedule{Duration: duration, Deadline: deadline, Period: domain.Period(f.Period)}, nil
	}
	return nil, domain.ErrScheduleRequired
}

func (f taskFields) fixedSchedule() (domain.Schedule, error) {
	days, err := calendar.ParseWeekdayNames(f.Weekdays)
	if err != nil {
		return nil, err
	}
	date, err := calendar.ParseOptionalDate(f.Date)
	if err != nil {
		return nil, err
	}
	start, err := calendar.ParseTimeOfDay(f.Start)
	if err != nil {
		return nil, err
	}
	end, err := calendar.ParseTimeOfDay(f.End)
	if err != nil {
		return nil, err
	}
	return domain.FixedSchedule{Weekdays: days, Date: date, Start: start, End: end}, nil
}

func toBody(t domain.Task) taskBody {
	f := taskFields{
		Title:    t.Title,
		Area:     t.Area,
		Kind:     string(t.Schedule.Kind()),
		Priority: string(t.Priority),
	}
	switch s := t.Schedule.(type) {
	case domain.FixedSchedule:
		f.Weekdays = s.Weekdays.Names()
		f.Date = s.Date.String()
		f.Start, f.End = s.Start.String(), s.End.String()
	case domain.RecurringSchedule:
		f.Weekdays = s.Weekdays.Names()
		f.DurationMinutes = int(s.Duration / time.Minute)
		f.Period = string(s.Period)
	case domain.OneOffSchedule:
		f.DurationMinutes = int(s.Duration / time.Minute)
		f.Deadline = s.Deadline.String()
		f.Period = string(s.Period)
	}
	return taskBody{ID: t.ID.String(), taskFields: f, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}
