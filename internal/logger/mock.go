package logger

import (
	"context"
	"log/slog"
)

type noopHandler struct{}

func (noopHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

func (noopHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

func (noopHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return noopHandler{}
}

func (noopHandler) WithGroup(_ string) slog.Handler {
	return noopHandler{}
}

func NewNoopLogger() *slog.Logger {
	return slog.New(noopHandler{})
}
