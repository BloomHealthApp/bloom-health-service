package telemetry

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Bucket boundaries in seconds, from 5 ms to 10 s.
var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type instruments struct {
	httpDuration metric.Float64Histogram
	dbDuration   metric.Float64Histogram
	syncOps      metric.Int64Counter
	chatTurns    metric.Int64Counter
}

var (
	instOnce sync.Once
	inst     instruments
)

// The global meter delegates to the provider installed later by Setup, so instruments may be created first.
func get() *instruments {
	instOnce.Do(func() {
		m := otel.Meter(ScopeName)
		inst.httpDuration, _ = m.Float64Histogram("http.server.request.duration", metric.WithUnit("s"),
			metric.WithDescription("Duration of HTTP server requests."), metric.WithExplicitBucketBoundaries(latencyBuckets...))
		inst.dbDuration, _ = m.Float64Histogram("db.client.operation.duration", metric.WithUnit("s"),
			metric.WithDescription("Duration of MongoDB commands."), metric.WithExplicitBucketBoundaries(latencyBuckets...))
		inst.syncOps, _ = m.Int64Counter("bloom.sync.operations", metric.WithUnit("{operation}"),
			metric.WithDescription("Pushed sync operations by entity type, op and result."))
		inst.chatTurns, _ = m.Int64Counter("bloom.chat.turns.enqueued", metric.WithUnit("{turn}"),
			metric.WithDescription("Assistant turns queued for bloom-health-task-worker."))
	})
	return &inst
}

// RecordHTTP records one finished request by method, route pattern and status.
func RecordHTTP(ctx context.Context, method, route string, status int, d time.Duration) {
	get().httpDuration.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("http.request.method", method), attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status)))
}

// RecordDB records one MongoDB command. errorType is "" on success.
func RecordDB(ctx context.Context, op, collection, errorType string, d time.Duration) {
	attrs := []attribute.KeyValue{attribute.String("db.system.name", "mongodb"), attribute.String("db.operation.name", op),
		attribute.String("db.collection.name", collection)}
	if errorType != "" {
		attrs = append(attrs, attribute.String("error.type", errorType))
	}
	get().dbDuration.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordSyncOp counts one pushed operation by its outcome (applied, duplicate, conflict, rejected).
func RecordSyncOp(ctx context.Context, entityType, op, result string) {
	get().syncOps.Add(ctx, 1, metric.WithAttributes(attribute.String("entity_type", entityType),
		attribute.String("op", op), attribute.String("result", result)))
}

// RecordChatTurnEnqueued counts one queued assistant turn by model id.
func RecordChatTurnEnqueued(ctx context.Context, modelID string) {
	get().chatTurns.Add(ctx, 1, metric.WithAttributes(attribute.String("model", modelID)))
}
