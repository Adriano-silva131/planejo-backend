package domain

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var ErrTaskNotFound = domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")

var (
	ErrTitleRequired = invalid("task_title_required", "Informe o título da tarefa.")
	ErrTitleTooLong  = invalid("task_title_too_long", "O título pode ter no máximo 120 caracteres.")
	ErrAreaTooLong   = invalid("task_area_too_long", "A área pode ter no máximo 50 caracteres.")

	ErrScheduleRequired  = invalid("task_kind_required", "Informe o tipo da tarefa: fixa, recorrente ou pontual.")
	ErrPriorityRequired  = invalid("task_priority_required", "Informe a prioridade da tarefa.")
	ErrInvalidPriority   = invalid("task_invalid_priority", "Prioridade inválida.")
	ErrFixedTaskPriority = invalid("fixed_task_priority", "Tarefa fixa não tem prioridade: ela sempre acontece.")

	ErrFixedTaskDays    = invalid("fixed_task_days", "Tarefa fixa precisa de dias da semana ou de uma data, não dos dois.")
	ErrWeekdaysRequired = invalid("task_weekdays_required", "Escolha ao menos um dia da semana.")
	ErrInvalidWeekday   = invalid("task_invalid_weekday", "Dia da semana inválido.")
	ErrInvalidTime      = invalid("task_invalid_time", "Horário inválido: use o formato HH:MM.")
	ErrInvalidTimeRange = invalid("task_invalid_time_range", "O horário de término precisa ser depois do início.")
	ErrInvalidDate      = invalid("task_invalid_date", "Data inválida: use o formato AAAA-MM-DD.")
	ErrInvalidPeriod    = invalid("task_invalid_period", "Período inválido: use manhã, tarde ou noite.")
	ErrDeadlineRequired = invalid("one_off_task_deadline", "Tarefa pontual precisa de prazo.")

	ErrInvalidSessionDuration = invalid("task_invalid_session_duration",
		"A duração de cada sessão precisa ficar entre 5 minutos e 12 horas.")
	ErrInvalidTotalDuration = invalid("task_invalid_total_duration",
		"A duração total precisa ficar entre 5 minutos e 100 horas.")
)

func invalid(code, message string) error {
	return domainerr.New(domainerr.Invalid, code, message)
}
