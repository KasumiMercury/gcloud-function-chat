package functions

import (
	"context"
	"fmt"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"log/slog"
	"os"
)

func InitTracing() *trace.TracerProvider {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		panic("GOOGLE_CLOUD_PROJECT must be set")
	}
	appName := os.Getenv("NAME")
	if appName == "" {
		appName = "patotta-stone-function-chat"
	}

	ctx := context.Background()

	var opts []otlptracehttp.Option
	if localOnly := os.Getenv("LOCAL_ONLY"); localOnly == "true" {
		// In local environment, TLS is not set up.
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	client := otlptracehttp.NewClient(opts...)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		slog.Error(
			fmt.Sprintf("Failed to create exporter: %v", err),
		)
	}

	resources, err := resource.New(
		ctx,
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(appName),
		),
	)
	if err != nil {
		slog.Error(
			fmt.Sprintf("Failed to create resource: %v", err),
		)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resources),
	)

	// Set the global TracerProvider to the SDK`s TracerProvider.
	otel.SetTracerProvider(tp)

	// W3C Trace Context propagator
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp
}