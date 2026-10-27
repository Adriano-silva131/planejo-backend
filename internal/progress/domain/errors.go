package domain

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var ErrCheckInNotFound = domainerr.New(domainerr.NotFound, "checkin_not_found", "Registro não encontrado.")

var (
	ErrFixedTaskCheckIn = invalid("fixed_task_checkin", "Tarefas fixas não precisam de registro: elas sempre acontecem.")
	ErrFutureCheckIn    = invalid("future_checkin", "Não dá para registrar um dia que ainda não chegou.")
	ErrOneOffOnly       = invalid("checkin_one_off_only", "Tempo e conclusão só valem para tarefas pontuais.")
	ErrInvalidMinutes   = invalid("checkin_invalid_minutes", "O tempo do dia precisa ficar entre 0 e 1440 minutos.")
)

func invalid(code, message string) error {
	return domainerr.New(domainerr.Invalid, code, message)
}
