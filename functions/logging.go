package functions

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"os"
)

type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	// TODO: Implement custom logic to extract values from the context and add them to the log.
	// This is intended for future extensions where specific context values need to be logged.
	return h.Handler.Handle(ctx, r)
}

func NewCustomLogger(ctx context.Context) *slog.Logger {
	svcName := os.Getenv("SERVICE_NAME")
	if svcName == "" {
		if os.Getenv("LOCAL_ONLY") != "true" {
			slog.Error(
				"SERVICE_NAME is not set",
				slog.String("error", "SERVICE_NAME must be set"),
			)
			panic("SERVICE_NAME must be set")
		} else {
			svcName = "fetch-chat-function"
		}
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

	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		// Add trace ID to the logger
		// Error handling when GOOGLE_CLOUD_PROJECT is undefined is already done in InitTracing()
		traceString := fmt.Sprintf("projects/%s/traces/%s", os.Getenv("GOOGLE_CLOUD_PROJECT"), sc.TraceID().String())
		logger = logger.With(
			slog.String("logging.googleapis.com/trace", traceString),
			slog.String("logging.googleapis.com/spanId", sc.SpanID().String()),
			slog.Bool("logging.googleapis.com/trace_sampled", sc.TraceFlags().IsSampled()),
		)
	}

	return logger
}
