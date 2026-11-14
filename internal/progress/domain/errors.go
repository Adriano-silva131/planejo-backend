package domain

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var (
	ErrCheckInNotFound = domainerr.New(domainerr.NotFound, "checkin_not_found", "Registro não encontrado.")
	// ErrTaskNotFound has the task module's code, so clients see one error.
	ErrTaskNotFound = domainerr.New(domainerr.NotFound, "task_not_found", "Tarefa não encontrada.")
)

var (
	ErrFixedTaskCheckIn = invalid("fixed_task_checkin", "Tarefas fixas não precisam de registro: elas sempre acontecem.")
	ErrFutureCheckIn    = invalid("future_checkin", "Não dá para registrar um dia que ainda não chegou.")
	ErrAlreadyCompleted = invalid("one_off_already_completed", "Esta tarefa já foi concluída. Reabra para mudar o dia.")
)

func invalid(code, message string) error {
	return domainerr.New(domainerr.Invalid, code, message)
}
