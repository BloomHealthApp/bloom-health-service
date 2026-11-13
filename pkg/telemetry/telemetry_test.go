package telemetry

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.opentelemetry.io/otel/trace"

	"github.com/BloomHealthApp/bloom-health-service/config"
)

func TestSetupWithoutEndpointIsANoop(t *testing.T) {
	shutdown, err := Setup(context.Background(), &config.Store{})
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))
	require.Equal(t, "", TraceParent(context.Background()))
	id, _ := TraceIDs(context.Background())
	require.Equal(t, "", id)
}

func TestTraceParentRoundTrip(t *testing.T) {
	_, _ = Setup(context.Background(), &config.Store{})
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled}))
	require.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", TraceParent(ctx))
	gotTrace, gotSpan := TraceIDs(ctx)
	require.Equal(t, tid.String(), gotTrace)
	require.Equal(t, sid.String(), gotSpan)
}

func TestMongoErrorTypeNeverUsesTheMessage(t *testing.T) {
	dup := mongo.CommandError{Code: 11000, Message: `E11000 duplicate key error dup key: { email: "anu@example.com" }`}
	require.Equal(t, "11000", errorType(fmt.Errorf("insert: %w", dup)))
	require.Equal(t, "error", errorType(fmt.Errorf("network down")))
}
