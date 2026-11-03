// Package helpers holds pure mappers from DAOs and BOs to response DTOs. They do no I/O.
package helpers

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/chatstatus"
)

// ChatSessionResponse renders a chat session; a nil ContextScopes becomes [].
func ChatSessionResponse(s *dao.ChatSession) response.ChatSessionResponse {
	scopes := s.ContextScopes
	if scopes == nil {
		scopes = []string{}
	}
	return response.ChatSessionResponse{ID: s.ID, Title: s.Title, ModelID: s.ModelID, ContextScopes: scopes,
		PendingMessageID: s.ActiveTurnID, CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339)}
}

// ChatMessageResponse renders a chat message with its wire status; an empty ErrorCode becomes null.
func ChatMessageResponse(m *dao.ChatMessage) response.ChatMessageResponse {
	var code *string
	if m.ErrorCode != "" {
		c := m.ErrorCode
		code = &c
	}
	return response.ChatMessageResponse{ID: m.ID, SessionID: m.SessionID, Idx: m.Idx, Role: m.Role, Content: m.Content,
		Status: chatstatus.Wire(m.Status), ModelID: m.ModelID, ErrorCode: code,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339)}
}
