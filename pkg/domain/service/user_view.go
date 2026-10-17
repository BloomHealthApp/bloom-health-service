package service

import (
	"context"

	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/models/dao"
	"github.com/bubu-bloom/service/pkg/enums/userrole"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
)

// buildUserResponse renders the contract's `user` object, resolving the owner's display name.
func buildUserResponse(ctx context.Context, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, u *dao.User, m *dao.Membership) response.UserResponse {
	ownerName, ownerAvatar := u.DisplayName, u.Avatar
	if members, err := memberships.MembersOfSpace(ctx, m.SpaceID); err == nil {
		for _, mm := range members {
			if mm.Role == userrole.Owner {
				if ou, err := users.ByID(ctx, mm.UserID); err == nil {
					ownerName, ownerAvatar = ou.DisplayName, ou.Avatar
				}
			}
		}
	}
	return response.UserResponse{ID: u.ID, Email: u.Email, Phone: u.Phone, DisplayName: u.DisplayName, Avatar: u.Avatar,
		Timezone: u.Timezone, Theme: u.Theme, Role: string(m.Role), SharedSpaceID: m.SpaceID, OwnerDisplayName: ownerName, OwnerAvatar: ownerAvatar}
}
