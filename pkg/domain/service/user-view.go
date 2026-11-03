package service

import (
	"context"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/helpers"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

// buildUserResponse looks up the space owner and renders the contract's `user` object. Lookup failures
// fall back to the user's own name and avatar.
func buildUserResponse(ctx context.Context, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, u *dao.User, m *dao.Membership) response.UserResponse {
	var owner *dao.User
	if members, err := memberships.MembersOfSpace(ctx, m.SpaceID); err == nil {
		for _, mm := range members {
			if mm.Role == userrole.Owner {
				if ou, err := users.ByID(ctx, mm.UserID); err == nil {
					owner = ou
				}
			}
		}
	}
	return helpers.UserResponse(u, m, owner)
}
