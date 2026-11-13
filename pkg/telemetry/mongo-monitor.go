package telemetry

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Handshake and auth commands are noise, and auth commands carry credentials.
var skippedCommands = map[string]bool{"hello": true, "isMaster": true, "ismaster": true, "saslStart": true,
	"saslContinue": true, "authenticate": true, "buildInfo": true, "endSessions": true, "getnonce": true}

type cmdKey struct {
	conn string
	id   int64
}

type cmdSpan struct {
	ctx        context.Context
	span       trace.Span
	op         string
	collection string
}

// MongoMonitor returns a command monitor that traces and times each MongoDB command. It records the
// command and collection names only: never the command document or the server's error message, which
// can echo document values (a duplicate-key error quotes the key, such as an email address).
func MongoMonitor() *event.CommandMonitor {
	var inFlight sync.Map
	finish := func(k cmdKey, failure error, d time.Duration) {
		v, ok := inFlight.LoadAndDelete(k)
		if !ok {
			return
		}
		cs := v.(*cmdSpan)
		errType := ""
		if failure != nil {
			errType = errorType(failure)
			cs.span.SetAttributes(attribute.String("error.type", errType))
			cs.span.SetStatus(codes.Error, "mongodb command failed")
		}
		RecordDB(cs.ctx, cs.op, cs.collection, errType, d)
		cs.span.End()
	}
	return &event.CommandMonitor{
		Started: func(ctx context.Context, e *event.CommandStartedEvent) {
			if skippedCommands[e.CommandName] {
				return
			}
			collection := ""
			if el, err := e.Command.IndexErr(0); err == nil {
				collection, _ = el.Value().StringValueOK()
			}
			ctx, span := Tracer().Start(ctx, e.CommandName+" "+collection, trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attribute.String("db.system.name", "mongodb"), attribute.String("db.operation.name", e.CommandName),
					attribute.String("db.collection.name", collection), attribute.String("db.namespace", e.DatabaseName)))
			inFlight.Store(cmdKey{e.ConnectionID, e.RequestID}, &cmdSpan{ctx: ctx, span: span, op: e.CommandName, collection: collection})
		},
		Succeeded: func(_ context.Context, e *event.CommandSucceededEvent) {
			finish(cmdKey{e.ConnectionID, e.RequestID}, nil, e.Duration)
		},
		Failed: func(_ context.Context, e *event.CommandFailedEvent) {
			finish(cmdKey{e.ConnectionID, e.RequestID}, e.Failure, e.Duration)
		},
	}
}

// errorType is the server error code, or "error" when there is none. Never the message.
func errorType(err error) string {
	var ce mongo.CommandError
	if errors.As(err, &ce) && ce.Code != 0 {
		return strconv.Itoa(int(ce.Code))
	}
	return "error"
}
