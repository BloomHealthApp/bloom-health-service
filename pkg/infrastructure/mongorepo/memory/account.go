package memory

import (
	"context"
	"strings"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type accountRepo struct {
	users        *userRepo
	spaces       *spaceRepo
	memberships  *membershipRepo
	entities     *entityRepo
	ops          *opRepo
	otps         *otpRepo
	sessions     *sessionRepo
	chatSessions *chatSessionRepo
	chatMessages *chatMessageRepo
}

// NewAccountRepo erases from the given memory repositories, which must come from this package's constructors.
func NewAccountRepo(users mongorepo.IUserRepo, spaces mongorepo.ISpaceRepo, memberships mongorepo.IMembershipRepo,
	entities mongorepo.IEntityRepo, ops mongorepo.IOpRepo, otps mongorepo.IOTPRepo, sessions mongorepo.ISessionRepo,
	chatSessions mongorepo.IChatSessionRepo, chatMessages mongorepo.IChatMessageRepo) mongorepo.IAccountRepo {
	return &accountRepo{users: users.(*userRepo), spaces: spaces.(*spaceRepo), memberships: memberships.(*membershipRepo),
		entities: entities.(*entityRepo), ops: ops.(*opRepo), otps: otps.(*otpRepo), sessions: sessions.(*sessionRepo),
		chatSessions: chatSessions.(*chatSessionRepo), chatMessages: chatMessages.(*chatMessageRepo)}
}

func (r *accountRepo) EraseSpace(_ context.Context, spaceID string) error {
	r.entities.mu.Lock()
	for k, e := range r.entities.entities {
		if e.SpaceID == spaceID {
			delete(r.entities.entities, k)
		}
	}
	delete(r.entities.seq, spaceID)
	r.entities.mu.Unlock()

	r.ops.mu.Lock()
	for k, o := range r.ops.ops {
		if o.SpaceID == spaceID {
			delete(r.ops.ops, k)
		}
	}
	r.ops.mu.Unlock()

	r.memberships.mu.Lock()
	for k, m := range r.memberships.m {
		if m.SpaceID == spaceID {
			delete(r.memberships.m, k)
		}
	}
	r.memberships.mu.Unlock()

	r.spaces.mu.Lock()
	delete(r.spaces.spaces, spaceID)
	r.spaces.mu.Unlock()
	return nil
}

func (r *accountRepo) EraseUser(_ context.Context, u *dao.User) error {
	r.chatMessages.mu.Lock()
	for k, m := range r.chatMessages.messages {
		if m.UserID == u.ID {
			delete(r.chatMessages.messages, k)
		}
	}
	r.chatMessages.mu.Unlock()

	r.chatSessions.mu.Lock()
	for k, s := range r.chatSessions.sessions {
		if s.UserID == u.ID {
			delete(r.chatSessions.sessions, k)
		}
	}
	r.chatSessions.mu.Unlock()

	r.sessions.mu.Lock()
	for k, s := range r.sessions.sessions {
		if s.UserID == u.ID {
			delete(r.sessions.sessions, k)
		}
	}
	r.sessions.mu.Unlock()

	r.otps.mu.Lock()
	for k, c := range r.otps.otps {
		if c.Email == strings.ToLower(u.Email) {
			delete(r.otps.otps, k)
		}
	}
	r.otps.mu.Unlock()

	r.memberships.mu.Lock()
	for k, m := range r.memberships.m {
		if m.UserID == u.ID {
			delete(r.memberships.m, k)
		}
	}
	r.memberships.mu.Unlock()

	r.users.mu.Lock()
	delete(r.users.users, u.ID)
	r.users.mu.Unlock()
	return nil
}
