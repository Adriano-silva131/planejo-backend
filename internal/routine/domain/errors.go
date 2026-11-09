package domain

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var (
	ErrAdjustmentNotFound = domainerr.New(domainerr.NotFound, "adjustment_not_found", "Esta ocorrência não tem ajuste.")
	ErrSessionNotFound    = domainerr.New(domainerr.NotFound, "session_not_found", "Não há sessão desta tarefa neste dia.")
	// ErrTaskNotFound has the task module's code, so clients see one error.
	ErrTaskNotFound = domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")
)

var (
	ErrNoOccurrences    = invalid("task_without_occurrences", "Só tarefas fixas têm ocorrências; encaixe uma sessão desta tarefa.")
	ErrFixedTaskSession = invalid("fixed_task_session", "Tarefa fixa já tem horário; ajuste a ocorrência dela.")
	ErrEndRequired      = invalid("adjustment_end_required", "Informe o fim: tarefa fixa não tem duração própria.")
	ErrNotAnOccurrence  = invalid("not_an_occurrence", "A tarefa não acontece neste dia.")
	ErrInvalidTimeRange = invalid("adjustment_invalid_time_range", "O início e o fim não podem ser iguais.")

	ErrSessionEndRequired  = invalid("session_end_required", "Informe o fim: tarefa pontual não tem duração.")
	ErrSessionPastMidnight = invalid("session_past_midnight", "A sessão passa da meia-noite. Escolha um horário mais cedo.")
)

func invalid(code, message string) error {
	return domainerr.New(domainerr.Invalid, code, message)
}
