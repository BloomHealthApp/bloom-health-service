package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/BloomHealthApp/bloom-health-service/config"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo/memory"
	"github.com/BloomHealthApp/bloom-health-service/pkg/telemetry"
)

const clientTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"

// The global meter binds its instruments to the first provider set, so the providers are installed once per
// process (also under -count=N) and the assertions tolerate data from earlier runs.
var (
	telOnce   sync.Once
	telSpans  *tracetest.SpanRecorder
	telReader *sdkmetric.ManualReader
)

func installTestTelemetry() {
	telOnce.Do(func() {
		telSpans = tracetest.NewSpanRecorder()
		telReader = sdkmetric.NewManualReader()
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(telSpans)))
		otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(telReader)))
	})
}

func TestTelemetryTracesRequestsAndRecordsMetrics(t *testing.T) {
	installTestTelemetry()
	spans, reader := telSpans, telReader
	_, err := telemetry.Setup(context.Background(), &config.Store{}) // propagator only
	require.NoError(t, err)

	e := setup(t)
	tok, _ := e.login("anu@example.com")
	sid := e.newChat(tok)

	body, _ := json.Marshal(map[string]any{"content": "stretch tips"})
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/chat/sessions/"+sid+"/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("traceparent", "00-"+clientTraceID+"-00f067aa0ba902b7-01")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	var sent map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&sent))
	resp.Body.Close()
	require.Equal(t, 202, resp.StatusCode)

	// the server span continues the client's trace and names the route pattern, never the raw path
	// the span ends just after the response is written, so wait for it
	var server sdktrace.ReadOnlySpan
	require.Eventually(t, func() bool {
		for _, s := range spans.Ended() {
			if s.Name() == "POST /v1/chat/sessions/:sid/messages" {
				server = s
			}
		}
		return server != nil
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, clientTraceID, server.SpanContext().TraceID().String())
	for _, a := range server.Attributes() {
		require.NotContains(t, a.Value.Emit(), sid, "span attribute %s leaks the path", a.Key)
	}

	// the queued turn carries the trace on to the worker
	asstID := sent["assistantMessage"].(map[string]any)["id"].(string)
	var traceParent string
	require.True(t, memory.SettleForTest(e.repos.ChatMessages, asstID, func(m *dao.ChatMessage) { traceParent = m.TraceParent }))
	require.True(t, strings.HasPrefix(traceParent, "00-"+clientTraceID+"-"), traceParent)

	st, out := e.do("POST", "/v1/sync/push", tok, map[string]any{"deviceId": "d1", "operations": []any{map[string]any{
		"operationId": "op-tel", "entityType": "habit", "entityId": "default-water", "op": "delete", "baseVersion": 999}}})
	require.Equal(t, 200, st, out)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	byName := map[string]metricdata.Aggregation{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			byName[m.Name] = m.Data
		}
	}
	httpHist := byName["http.server.request.duration"].(metricdata.Histogram[float64])
	require.True(t, hasPoint(httpHist.DataPoints, "http.route", "/v1/chat/sessions/:sid/messages"))
	chat := byName["bloom.chat.turns.enqueued"].(metricdata.Sum[int64])
	require.GreaterOrEqual(t, chat.DataPoints[0].Value, int64(1))
	syncOps := byName["bloom.sync.operations"].(metricdata.Sum[int64])
	found := false
	for _, p := range syncOps.DataPoints {
		et, _ := p.Attributes.Value(attribute.Key("entity_type"))
		res, _ := p.Attributes.Value(attribute.Key("result"))
		found = found || (et.AsString() == "habit" && res.AsString() == "conflict")
	}
	require.True(t, found, "a conflicting habit push is counted")
}

func hasPoint(points []metricdata.HistogramDataPoint[float64], key, want string) bool {
	for _, p := range points {
		if v, ok := p.Attributes.Value(attribute.Key(key)); ok && v.AsString() == want {
			return true
		}
	}
	return false
}
