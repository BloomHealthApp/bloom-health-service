package helpers

import (
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
)

// UserResponse renders the contract's `user` object. owner is the space owner; nil falls back to u.
func UserResponse(u *dao.User, m *dao.Membership, owner *dao.User) response.UserResponse {
	if owner == nil {
		owner = u
	}
	return response.UserResponse{ID: u.ID, Email: u.Email, Phone: u.Phone, DisplayName: u.DisplayName, Avatar: u.Avatar,
		Timezone: u.Timezone, Theme: u.Theme, Role: string(m.Role), SharedSpaceID: m.SpaceID,
		OwnerDisplayName: owner.DisplayName, OwnerAvatar: owner.Avatar,
		Gender: u.Gender, PeriodTrackerEnabled: u.IsPeriodTrackerEnabled()}
}
