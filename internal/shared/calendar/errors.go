package calendar

import "github.com/Adriano-silva131/planejo-backend/internal/shared/domainerr"

var (
	ErrInvalidWeekday = domainerr.New(domainerr.Invalid, "invalid_weekday", "Dia da semana inválido.")
	ErrInvalidTime    = domainerr.New(domainerr.Invalid, "invalid_time", "Horário inválido: use o formato HH:MM.")
	ErrInvalidDate    = domainerr.New(domainerr.Invalid, "invalid_date", "Data inválida: use o formato AAAA-MM-DD.")
)
