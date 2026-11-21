package helpers

import (
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

// PublicProfileResponse renders only the public fields of u; stats are left out when nil.
func PublicProfileResponse(u *dao.User, stats *bo.ProfileStats) response.PublicProfileResponse {
	p := response.PublicProfileResponse{ID: u.ID, DisplayName: u.DisplayName, Avatar: u.Avatar}
	if stats != nil {
		p.Stats = &response.PublicStatsResponse{CurrentStreak: stats.CurrentStreak, LongestStreak: stats.LongestStreak, Habits: stats.Habits}
	}
	return p
}

// ConnectionResponse renders a connection as seen by the other user's profile; friends date from acceptance.
func ConnectionResponse(c *dao.Connection, other *dao.User) response.ConnectionResponse {
	since := c.CreatedAt
	if c.Status == dao.ConnectionAccepted && c.RespondedAt != nil {
		since = *c.RespondedAt
	}
	return response.ConnectionResponse{ID: c.ID, Profile: PublicProfileResponse(other, nil), Since: since.UTC().Format(time.RFC3339)}
}
