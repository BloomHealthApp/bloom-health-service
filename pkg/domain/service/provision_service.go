package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/serviceBos"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/theme"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/validation"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
)

// IProvisionService creates users, spaces, memberships, settings and the default habits idempotently.
type IProvisionService interface {
	// EnsureAccount returns the user for a verified email, creating the user (with the given display
	// name and E.164 phone) plus a private space (owner membership, default habits, default settings)
	// on first call. An empty avatarID or genderID stores none (null). An empty name falls back to the email's local part; an empty phone stores none.
	// Returns a wrapped ErrRepoConflict when the email or phone belongs to someone else. Safe to repeat.
	EnsureAccount(ctx context.Context, name, phone, avatarID, genderID, email, timezone string) (*dao.User, error)
	// Provision creates an owner and a viewer sharing one space. It is the only way to create a viewer
	// membership until invitations exist; no endpoint exposes it.
	Provision(ctx context.Context, opts serviceBos.ProvisionOptions) error
}

type provisionService struct {
	log         logger.ILogger
	users       mongorepo.IUserRepo
	spaces      mongorepo.ISpaceRepo
	memberships mongorepo.IMembershipRepo
	entities    mongorepo.IEntityRepo
	clock       clock.Clock
}

// NewProvisionService wires the provisioning service.
func NewProvisionService(log logger.ILogger, users mongorepo.IUserRepo, spaces mongorepo.ISpaceRepo,
	memberships mongorepo.IMembershipRepo, entities mongorepo.IEntityRepo, clk clock.Clock) IProvisionService {
	return &provisionService{log: log, users: users, spaces: spaces, memberships: memberships, entities: entities, clock: clk}
}

type habitDef struct {
	id, title, icon, category, tracking, comparison, unit string
	target                                                float64
	targetMax                                             *float64
}

func fp(f float64) *float64 { return &f }

var defaultHabits = []habitDef{
	{"default-wakeup", "Wake up by 5:00 AM", "sunrise", "morning", "timeOfDay", "by", "", 300, nil},
	{"default-exercise", "Exercise 90–120 min", "dumbbell", "fitness", "duration", "between", "min", 90, fp(120)},
	{"default-no-sugar", "No added sugar", "candy-off", "nutrition", "boolean", "atLeast", "", 1, nil},
	{"default-no-junk", "No junk food", "salad", "nutrition", "boolean", "atLeast", "", 1, nil},
	{"default-dinner", "Dinner by 6:00 PM", "utensils", "nutrition", "timeOfDay", "by", "", 1080, nil},
	{"default-water", "Water 2,000 ml", "droplet", "hydration", "quantity", "atLeast", "ml", 2000, nil},
	{"default-steps", "Steps 7,000–10,000", "footprints", "fitness", "quantity", "atLeast", "steps", 7000, fp(10000)},
	{"default-sleep", "Bedtime by 10:00 PM", "moon", "sleep", "timeOfDay", "by", "", 1320, nil},
}

func (s *provisionService) ensureUser(ctx context.Context, p serviceBos.Person, phone, avatarID, genderID *string, tz string, now time.Time) (*dao.User, error) {
	email := request.NormalizeEmail(p.Email)
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("invalid email for %s", p.Name)
	}
	u, err := s.users.ByEmail(ctx, email)
	if err == nil {
		return u, nil
	}
	if !apperrors.IsNotFound(err) {
		return nil, err
	}
	u = &dao.User{ID: uuid.NewString(), Email: email, DisplayName: p.Name, Phone: phone, Avatar: avatarID, Gender: genderID, Timezone: tz, Theme: theme.CottonCandy, CreatedAt: now, UpdatedAt: now}
	return u, s.users.Save(ctx, u)
}

func (s *provisionService) Provision(ctx context.Context, o serviceBos.ProvisionOptions) error {
	now := s.clock.Now().UTC()
	if o.Timezone == "" {
		o.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(o.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q", o.Timezone)
	}
	owner, err := s.ensureUser(ctx, o.Owner, nil, nil, nil, o.Timezone, now)
	if err != nil {
		return err
	}
	viewer, err := s.ensureUser(ctx, o.Viewer, nil, nil, nil, o.Timezone, now)
	if err != nil {
		return err
	}
	var spaceID string
	if m, err := s.memberships.ForUser(ctx, owner.ID); err == nil {
		spaceID = m.SpaceID
	} else if apperrors.IsNotFound(err) {
		spaceID = uuid.NewString()
		name := o.SpaceName
		if name == "" {
			name = owner.DisplayName + " & " + viewer.DisplayName
		}
		if err := s.spaces.Save(ctx, &dao.Space{ID: spaceID, Name: name, CreatedBy: owner.ID, CreatedAt: now}); err != nil {
			return err
		}
		if err := s.memberships.Save(ctx, &dao.Membership{ID: uuid.NewString(), SpaceID: spaceID, UserID: owner.ID, Role: userrole.Owner, CreatedAt: now}); err != nil {
			return err
		}
	} else {
		return err
	}
	if _, err := s.memberships.ForUser(ctx, viewer.ID); apperrors.IsNotFound(err) {
		if err := s.memberships.Save(ctx, &dao.Membership{ID: uuid.NewString(), SpaceID: spaceID, UserID: viewer.ID, Role: userrole.Viewer, CreatedAt: now}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return s.seedSpace(ctx, spaceID, now)
}

// seedSpace writes the default habits and settings for a space if they are missing.
func (s *provisionService) seedSpace(ctx context.Context, spaceID string, now time.Time) error {
	for i, h := range defaultHabits {
		if _, err := s.entities.Get(ctx, spaceID, entitytype.Habit, h.id); err == nil {
			continue
		}
		var unit any
		if h.unit != "" {
			unit = h.unit
		}
		var tmax any
		if h.targetMax != nil {
			tmax = *h.targetMax
		}
		data, err := validation.Merge(entitytype.Habit, nil, map[string]any{
			"title": h.title, "icon": h.icon, "category": h.category, "trackingType": h.tracking,
			"targetComparison": h.comparison, "target": h.target, "targetMax": tmax, "unit": unit,
			"sortOrder": float64(i),
		}, validation.Context{})
		if err != nil {
			return err
		}
		data["isDefault"] = true
		e := &dao.Entity{ID: h.id, Type: entitytype.Habit, SpaceID: spaceID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: data}
		if err := s.entities.Save(ctx, e); err != nil && !apperrors.IsConflict(err) {
			return err
		}
	}
	if _, err := s.entities.Get(ctx, spaceID, entitytype.Settings, constants.SettingsID); apperrors.IsNotFound(err) {
		data, err := validation.Merge(entitytype.Settings, nil, validation.DefaultSettings(), validation.Context{})
		if err != nil {
			return err
		}
		e := &dao.Entity{ID: constants.SettingsID, Type: entitytype.Settings, SpaceID: spaceID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: data}
		if err := s.entities.Save(ctx, e); err != nil && !apperrors.IsConflict(err) {
			return err
		}
	}
	return nil
}

func (s *provisionService) EnsureAccount(ctx context.Context, name, phone, avatarID, genderID, email, timezone string) (*dao.User, error) {
	now := s.clock.Now().UTC()
	email = request.NormalizeEmail(email)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		timezone = "UTC"
	}
	if name = strings.TrimSpace(name); name == "" {
		name = email
		if i := strings.Index(email, "@"); i > 0 {
			name = email[:i]
		}
	}
	if r := []rune(name); len(r) > 40 {
		name = string(r[:40])
	}
	var ph *string
	if phone != "" {
		ph = &phone
	}
	var av *string
	if avatarID != "" {
		av = &avatarID
	}
	var gd *string
	if genderID != "" {
		gd = &genderID
	}
	u, err := s.ensureUser(ctx, serviceBos.Person{Email: email, Name: name}, ph, av, gd, timezone, now)
	if err != nil {
		return nil, err
	}
	if _, err := s.memberships.ForUser(ctx, u.ID); err == nil {
		return u, nil
	} else if !apperrors.IsNotFound(err) {
		return nil, err
	}
	spaceID := uuid.NewString()
	if err := s.spaces.Save(ctx, &dao.Space{ID: spaceID, Name: u.DisplayName, CreatedBy: u.ID, CreatedAt: now}); err != nil {
		return nil, err
	}
	if err := s.memberships.Save(ctx, &dao.Membership{ID: uuid.NewString(), SpaceID: spaceID, UserID: u.ID, Role: userrole.Owner, CreatedAt: now}); err != nil {
		return nil, err
	}
	if err := s.seedSpace(ctx, spaceID, now); err != nil {
		return nil, err
	}
	return u, nil
}
