package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatstatus"
)

func TestChatSessionResponse(t *testing.T) {
	at := time.Date(2026, 11, 3, 10, 0, 0, 0, time.FixedZone("IST", 19800))
	turn := "m2"
	tests := []struct {
		name       string
		in         dao.ChatSession
		wantScopes []string
	}{
		{"nil scopes become empty", dao.ChatSession{ID: "s1", CreatedAt: at, UpdatedAt: at}, []string{}},
		{"scopes kept", dao.ChatSession{ID: "s1", ContextScopes: []string{"habits"}, ActiveTurnID: &turn, CreatedAt: at, UpdatedAt: at}, []string{"habits"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChatSessionResponse(&tt.in)
			assert.Equal(t, tt.wantScopes, got.ContextScopes)
			assert.Equal(t, tt.in.ActiveTurnID, got.PendingMessageID)
			assert.Equal(t, "2026-11-03T04:30:00Z", got.CreatedAt)
		})
	}
}

func TestChatMessageResponse(t *testing.T) {
	tests := []struct {
		name       string
		in         dao.ChatMessage
		wantStatus string
		wantCode   *string
	}{
		{"running is pending on the wire", dao.ChatMessage{Status: chatstatus.Running}, chatstatus.Pending, nil},
		{"failed keeps its code", dao.ChatMessage{Status: chatstatus.Failed, ErrorCode: "model_error"}, chatstatus.Failed, ptr("model_error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChatMessageResponse(&tt.in)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantCode, got.ErrorCode)
		})
	}
}

func ptr(s string) *string { return &s }
