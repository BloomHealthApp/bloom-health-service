package service

import (
	"context"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

// eraseAccount hard-deletes u and, when u owns a space, the whole space with every viewer's access to it. The space
// goes first: if a later step fails, the user still exists and the erase can be retried.
func eraseAccount(ctx context.Context, accounts mongorepo.IAccountRepo, memberships mongorepo.IMembershipRepo, u *dao.User) error {
	m, err := memberships.ForUser(ctx, u.ID)
	if err != nil && !apperrors.IsNotFound(err) {
		return err
	}
	if err == nil && m.Role == userrole.Owner {
		if err := accounts.EraseSpace(ctx, m.SpaceID); err != nil {
			return err
		}
	}
	return accounts.EraseUser(ctx, u)
}

// overdue reports whether u's grace period has ended, so the account must be erased instead of used.
func overdue(u *dao.User, now time.Time) bool {
	return u.DeleteAfter != nil && !now.Before(*u.DeleteAfter)
}
