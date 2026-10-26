package logger

import (
	"io"
	"log/slog"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/config"
)

func New(w io.Writer, env config.Environment, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if env == config.Production {
		return slog.New(contextHandler{slog.NewJSONHandler(w, opts)})
	}
	return slog.New(contextHandler{slog.NewTextHandler(w, opts)})
}
