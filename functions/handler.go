package functions

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
)

type Flush interface {
	ForceFlush(ctx context.Context) error
}

type HttpHandler func(http.ResponseWriter, *http.Request)

func InstrumentedHandler(name string, function HttpHandler, flusher Flush) HttpHandler {
	opts := []trace.SpanStartOption{
		// customizable span attributes
		trace.WithAttributes(semconv.FaaSTriggerHTTP),
	}

	// create instrumented handler
	handler := otelhttp.NewHandler(
		http.HandlerFunc(function), name, otelhttp.WithSpanOptions(opts...),
	)

	return func(w http.ResponseWriter, r *http.Request) {
		// call the actual handler
		handler.ServeHTTP(w, r)

		// force flush the span data
		err := flusher.ForceFlush(r.Context())
		if err != nil {
			// If ForceFlush() execution fails, spans are sent to the background and may be missing,
			// but are tolerated and ignored.
			slog.Error(
				"Failed to flush spans",
				slog.Group("tracing", slog.Group("forceFlush", "error", err)),
			)
		}
	}
}
