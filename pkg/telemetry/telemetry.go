// Package telemetry sets up OpenTelemetry traces and metrics exported over OTLP/HTTP. With no
// OTEL_EXPORTER_OTLP_ENDPOINT it installs nothing but the W3C trace-context propagator, so spans and
// instruments stay no-ops and tests pay no cost. Attributes are limited to routes, methods, statuses,
// collection and command names: never paths, queries, bodies, document values or error messages.
package telemetry

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/BloomHealthApp/bloom-health-service/config"
)

// ScopeName names the tracer and meter of this service.
const ScopeName = "github.com/BloomHealthApp/bloom-health-service"

// Shutdown flushes and stops the exporters.
type Shutdown func(context.Context) error

// Setup installs the global propagator and, when cfg.OTLPEndpoint is set, OTLP trace and metric pipelines.
// The exporters read the standard OTEL_EXPORTER_OTLP_* variables (endpoint, headers) themselves.
func Setup(ctx context.Context, cfg *config.Store) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	if cfg.OTLPEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "bloom-health-service"),
			attribute.String("service.version", cfg.Version),
			attribute.String("deployment.environment.name", cfg.ActiveEnv),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, err
	}
	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	// Short batch and export intervals: on Vercel an idle instance may be frozen before a long timer fires.
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp, sdktrace.WithBatchTimeout(2*time.Second)))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second))))
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	return func(ctx context.Context) error { return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx)) }, nil
}

// Tracer is the service's tracer (delegates to whatever provider Setup installed).
func Tracer() trace.Tracer { return otel.Tracer(ScopeName) }

// TraceParent returns the W3C traceparent of the span in ctx, or "" when there is none. It is stored on
// queued chat turns so bloom-health-task-worker can continue the trace.
func TraceParent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// TraceIDs returns the trace and span ids in ctx for log correlation ("" when there is no valid span).
func TraceIDs(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}
