package service

import (
	"context"
	"time"

	"github.com/bubu-bloom/service/pkg/constants"
	"github.com/bubu-bloom/service/pkg/domain/dto/request"
	"github.com/bubu-bloom/service/pkg/domain/dto/response"
	"github.com/bubu-bloom/service/pkg/domain/serviceBos"
	"github.com/bubu-bloom/service/pkg/enums/entitytype"
	"github.com/bubu-bloom/service/pkg/enums/userrole"
	apperrors "github.com/bubu-bloom/service/pkg/errors"
	"github.com/bubu-bloom/service/pkg/infrastructure/mongorepo"
	"github.com/bubu-bloom/service/pkg/logger"
	"github.com/bubu-bloom/service/pkg/utils"
	"github.com/bubu-bloom/service/pkg/utils/clock"
	"github.com/bubu-bloom/service/pkg/utils/scoring"
)

// IStatsService computes streaks and completion server-side with the same algorithm as the app.
type IStatsService interface {
	Summary(ctx context.Context, actor *serviceBos.Actor, req *request.RangeReq) (*response.SummaryResponse, apperrors.IHttpError)
}

type statsService struct {
	log         logger.ILogger
	entities    mongorepo.IEntityRepo
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	clock       clock.Clock
}

// NewStatsService wires the stats service.
func NewStatsService(log logger.ILogger, entities mongorepo.IEntityRepo, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, clk clock.Clock) IStatsService {
	return &statsService{log: log, entities: entities, users: users, memberships: memberships, clock: clk}
}

// ownerTimezone is the timezone that defines "today" for a space (the owner's).
func (s *statsService) ownerTimezone(ctx context.Context, spaceID string) string {
	members, _ := s.memberships.MembersOfSpace(ctx, spaceID)
	for _, m := range members {
		if m.Role == userrole.Owner {
			if u, err := s.users.ByID(ctx, m.UserID); err == nil {
				return u.Timezone
			}
		}
	}
	return "UTC"
}

func (s *statsService) Summary(ctx context.Context, a *serviceBos.Actor, req *request.RangeReq) (*response.SummaryResponse, apperrors.IHttpError) {
	sum, err := s.summary(ctx, a.SpaceID(), req)
	if err != nil {
		return nil, internalErr(s.log, ctx, "stats failed", err)
	}
	return sum, nil
}

func (s *statsService) summary(ctx context.Context, spaceID string, req *request.RangeReq) (*response.SummaryResponse, error) {
	tz := s.ownerTimezone(ctx, spaceID)
	today := scoring.LocalDate(s.clock.Now(), tz)
	from, to := req.From, req.To
	if to == "" {
		to = today
	}
	if from == "" {
		t, _ := time.Parse(utils.DateLayout, to)
		from = t.AddDate(0, 0, -29).Format(utils.DateLayout)
	}
	habitEnts, err := s.entities.List(ctx, spaceID, entitytype.Habit, mongorepo.EntityQuery{})
	if err != nil {
		return nil, err
	}
	var habits []scoring.Habit
	for i := range habitEnts {
		habits = append(habits, habitFromEntity(&habitEnts[i]))
	}
	logEnts, err := s.entities.List(ctx, spaceID, entitytype.DailyLog, mongorepo.EntityQuery{})
	if err != nil {
		return nil, err
	}
	byDate := map[string]map[string]scoring.Log{}
	earliest := today
	for _, l := range logEnts {
		d, _ := l.Data["localDate"].(string)
		hid, _ := l.Data["habitId"].(string)
		if d == "" || hid == "" {
			continue
		}
		if byDate[d] == nil {
			byDate[d] = map[string]scoring.Log{}
		}
		lg := scoring.Log{}
		if v, ok := l.Data["value"].(float64); ok {
			lg.Value = &v
		}
		lg.Status, _ = l.Data["status"].(string)
		byDate[d][hid] = lg
		if d < earliest {
			earliest = d
		}
	}
	settings := scoring.Settings{Threshold: 80, RestMode: "neutral"}
	if se, err := s.entities.Get(ctx, spaceID, entitytype.Settings, constants.SettingsID); err == nil {
		if v, ok := se.Data["dailyThreshold"].(float64); ok {
			settings.Threshold = v
		}
		if v, ok := se.Data["restDayMode"].(string); ok {
			settings.RestMode = v
		}
		if arr, ok := se.Data["restDays"].([]any); ok {
			for _, x := range arr {
				if f, ok := x.(float64); ok {
					settings.RestDays = append(settings.RestDays, int(f))
				}
			}
		}
	}
	all, err := scoring.ComputeDays(habits, byDate, settings, earliest, today, today)
	if err != nil {
		return nil, err
	}
	cur, long := scoring.Streaks(all, today)
	window := []scoring.Day{}
	for _, d := range all {
		if d.Date >= from && d.Date <= to {
			window = append(window, d)
		}
	}
	sum := &response.SummaryResponse{CurrentStreak: cur, LongestStreak: long, Days: window, Milestones: scoring.MilestonesReached(long)}
	workouts, err := s.entities.List(ctx, spaceID, entitytype.Workout, mongorepo.EntityQuery{From: from, To: to})
	if err != nil {
		return nil, err
	}
	for _, w := range workouts {
		if _, ended := w.Data["endedAt"].(string); !ended {
			continue
		}
		sum.WorkoutCount++
		if d, ok := w.Data["durationSeconds"].(float64); ok {
			sum.WorkoutSecs += int(d)
		}
	}
	return sum, nil
}
