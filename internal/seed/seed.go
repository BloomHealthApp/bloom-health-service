// Package seed provisions the invite-only users, shared space and default habits idempotently.
package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bubu-bloom/service/internal/auth"
	"github.com/bubu-bloom/service/internal/entities"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/store"
)

type Person struct{ Email, Name string }

type Options struct {
	Owner, Viewer Person
	Timezone      string
	SpaceName     string
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

func ensureUser(ctx context.Context, st store.Store, p Person, tz string, now time.Time) (*model.User, error) {
	email := auth.NormalizeEmail(p.Email)
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("invalid email for %s", p.Name)
	}
	u, err := st.UserByEmail(ctx, email)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	u = &model.User{ID: uuid.NewString(), Email: email, DisplayName: p.Name, Timezone: tz, Theme: "cottonCandy", CreatedAt: now, UpdatedAt: now}
	return u, st.SaveUser(ctx, u)
}

// Provision creates users, the space, memberships, settings and the eight default habits. Safe to re-run.
func Provision(ctx context.Context, st store.Store, o Options, now time.Time) error {
	if o.Timezone == "" {
		o.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(o.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q", o.Timezone)
	}
	owner, err := ensureUser(ctx, st, o.Owner, o.Timezone, now)
	if err != nil {
		return err
	}
	viewer, err := ensureUser(ctx, st, o.Viewer, o.Timezone, now)
	if err != nil {
		return err
	}
	var spaceID string
	if m, err := st.MembershipForUser(ctx, owner.ID); err == nil {
		spaceID = m.SpaceID
	} else if errors.Is(err, store.ErrNotFound) {
		spaceID = uuid.NewString()
		name := o.SpaceName
		if name == "" {
			name = owner.DisplayName + " & " + viewer.DisplayName
		}
		if err := st.SaveSpace(ctx, &model.Space{ID: spaceID, Name: name, CreatedBy: owner.ID, CreatedAt: now}); err != nil {
			return err
		}
		if err := st.SaveMembership(ctx, &model.Membership{ID: uuid.NewString(), SpaceID: spaceID, UserID: owner.ID, Role: model.RoleOwner, CreatedAt: now}); err != nil {
			return err
		}
	} else {
		return err
	}
	if _, err := st.MembershipForUser(ctx, viewer.ID); errors.Is(err, store.ErrNotFound) {
		if err := st.SaveMembership(ctx, &model.Membership{ID: uuid.NewString(), SpaceID: spaceID, UserID: viewer.ID, Role: model.RoleViewer, CreatedAt: now}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	for i, h := range defaultHabits {
		if _, err := st.GetEntity(ctx, spaceID, model.TypeHabit, h.id); err == nil {
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
		data, err := entities.Merge(model.TypeHabit, nil, map[string]any{
			"title": h.title, "icon": h.icon, "category": h.category, "trackingType": h.tracking,
			"targetComparison": h.comparison, "target": h.target, "targetMax": tmax, "unit": unit,
			"sortOrder": float64(i),
		}, entities.Context{})
		if err != nil {
			return err
		}
		data["isDefault"] = true
		e := &model.Entity{ID: h.id, Type: model.TypeHabit, SpaceID: spaceID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: data}
		if err := st.SaveEntity(ctx, e); err != nil && !errors.Is(err, store.ErrConflict) {
			return err
		}
	}
	if _, err := st.GetEntity(ctx, spaceID, model.TypeSettings, model.SettingsID); errors.Is(err, store.ErrNotFound) {
		data, err := entities.Merge(model.TypeSettings, nil, entities.DefaultSettings(), entities.Context{})
		if err != nil {
			return err
		}
		e := &model.Entity{ID: model.SettingsID, Type: model.TypeSettings, SpaceID: spaceID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: data}
		if err := st.SaveEntity(ctx, e); err != nil && !errors.Is(err, store.ErrConflict) {
			return err
		}
	}
	return nil
}
