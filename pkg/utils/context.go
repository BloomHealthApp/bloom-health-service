// Package utils holds small helpers shared by all layers: request-context accessors, dates, cursors.
package utils

import (
	"context"

	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
)

type ctxKey string

// Keys of values stored in the request context.
const (
	RequestIDKey  ctxKey = "requestId"
	DeviceIDKey   ctxKey = "deviceId"
	ClientTypeKey ctxKey = "clientType"
	AppVersionKey ctxKey = "appVersion"
	TimezoneKey   ctxKey = "timezone"
	ActorKey      ctxKey = "actor"
)

// SetInContext returns a context carrying value under key.
func SetInContext(ctx context.Context, key ctxKey, value any) context.Context {
	return context.WithValue(ctx, key, value)
}

// GetFromContext returns the string stored under key, or "".
func GetFromContext(ctx context.Context, key ctxKey) string {
	s, _ := ctx.Value(key).(string)
	return s
}

// GetActor returns the authenticated actor, or nil when the request is unauthenticated.
func GetActor(ctx context.Context) *serviceBos.Actor {
	a, _ := ctx.Value(ActorKey).(*serviceBos.Actor)
	return a
}
