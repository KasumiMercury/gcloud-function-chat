package functions

import (
	"context"
	"log/slog"
	"os"
)

type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add custom logic here
	return h.Handler.Handle(ctx, r)
}

func NewCustomLogger(ctx context.Context) *slog.Logger {
	svcName := os.Getenv("SERVICE_NAME")
	if svcName == "" {
		svcName = "local"
	}

	handler := CustomHandler{
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				AddSource: true,
				Level:     slog.LevelInfo,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					switch a.Key {
					case slog.MessageKey:
						a = slog.Attr{
							Key:   "message",
							Value: a.Value,
						}
					case slog.LevelKey:
						a = slog.Attr{
							Key:   "severity",
							Value: a.Value,
						}
					case slog.SourceKey:
						a = slog.Attr{
							Key:   "logging.googleapis.com/sourceLocation",
							Value: a.Value,
						}
					}
					return a
				},
			}),
	}

	logger := slog.New(&handler).With(
		slog.Group("logging.googleapis.com/labels",
			slog.String("service", svcName),
		))

	return logger
}
