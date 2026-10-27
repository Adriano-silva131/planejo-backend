package domain

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var (
	ErrAdjustmentNotFound = domainerr.New(domainerr.NotFound, "adjustment_not_found", "Esta ocorrência não tem ajuste.")
	// ErrTaskNotFound has the task module's code, so clients see one error.
	ErrTaskNotFound = domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")
)

var (
	ErrNotFixedTask     = invalid("not_fixed_task", "Só tarefas fixas fazem parte da rotina.")
	ErrNotAnOccurrence  = invalid("not_an_occurrence", "A tarefa não acontece neste dia.")
	ErrInvalidTimeRange = invalid("adjustment_invalid_time_range", "O início e o fim não podem ser iguais.")
)

func invalid(code, message string) error {
	return domainerr.New(domainerr.Invalid, code, message)
}
