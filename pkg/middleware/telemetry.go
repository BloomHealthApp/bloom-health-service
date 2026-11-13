package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/BloomHealthApp/bloom-health-service/pkg/telemetry"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
)

// Telemetry continues the client's W3C traceparent (if any), opens one server span per request and records
// the request duration. Like RequestLogger it records the route pattern, never paths, queries or bodies.
func Telemetry() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		route := routeOf(c)
		method := c.Request.Method
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := telemetry.Tracer().Start(ctx, method+" "+route, trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route),
				attribute.String("bloom.request_id", utils.GetFromContext(ctx, utils.RequestIDKey))))
		c.Request = c.Request.WithContext(ctx)
		defer span.End()

		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.response.status_code", status))
		if status >= 500 {
			span.SetStatus(codes.Error, "")
		}
		telemetry.RecordHTTP(ctx, method, route, status, time.Since(start))
	}
}
