package service

import (
	"context"
	"sort"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/constants"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/bo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/request"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/dto/response"
	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/entitytype"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/validation"
	"github.com/BloomHealthApp/bloom-health-service/pkg/logger"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/clock"
	"github.com/BloomHealthApp/bloom-health-service/pkg/utils/scoring"
)

// IStatsService computes streaks and completion server-side with the same algorithm as the app.
type IStatsService interface {
	Summary(ctx context.Context, actor *bo.Actor, req *request.RangeReq) (*response.SummaryResponse, apperrors.IHttpError)
	// ProfileStats returns the streaks and active habit count of the space userID owns, for their public profile.
	// It is nil, with no error, when they own no space (a viewer).
	ProfileStats(ctx context.Context, userID string) (*bo.ProfileStats, error)
	// ProfileActivity returns a year of daily heat levels and today's habits for the space userID owns, for friends.
	// It is nil, with no error, when they own no space (a viewer).
	ProfileActivity(ctx context.Context, userID string) (*bo.ProfileActivity, error)
}

type statsService struct {
	log         logger.ILogger
	entities    mongorepo.IEntityRepo
	users       mongorepo.IUserRepo
	memberships mongorepo.IMembershipRepo
	clock       clock.Clock
}

func NewStatsService(log logger.ILogger, entities mongorepo.IEntityRepo, users mongorepo.IUserRepo, memberships mongorepo.IMembershipRepo, clk clock.Clock) IStatsService {
	return &statsService{log: log, entities: entities, users: users, memberships: memberships, clock: clk}
}

// spaceOwner is the owner of a space (nil if it cannot be resolved). Its timezone defines "today"
// and its period tracker setting decides whether period days are neutral.
func (s *statsService) spaceOwner(ctx context.Context, spaceID string) *dao.User {
	members, _ := s.memberships.MembersOfSpace(ctx, spaceID)
	for _, m := range members {
		if m.Role == userrole.Owner {
			if u, err := s.users.ByID(ctx, m.UserID); err == nil {
				return u
			}
		}
	}
	return nil
}

// periodNeutralDates expands the owner's non-deleted periods into YYYY-MM-DD dates (an ongoing period
// lasts through today). Only the dates leave this function; no flow, symptoms, mood or notes.
func periodNeutralDates(periods []dao.Entity, today string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range periods {
		start, end := validation.PeriodRange(p.Data)
		st, err := time.Parse(utils.DateLayout, start)
		if err != nil {
			continue
		}
		if end == "" || end > today {
			end = today
		}
		en, err := time.Parse(utils.DateLayout, end)
		if err != nil {
			continue
		}
		for d := st; !d.After(en); d = d.AddDate(0, 0, 1) {
			ds := d.Format(utils.DateLayout)
			if !seen[ds] {
				seen[ds] = true
				out = append(out, ds)
			}
		}
	}
	return out
}

func (s *statsService) Summary(ctx context.Context, a *bo.Actor, req *request.RangeReq) (*response.SummaryResponse, apperrors.IHttpError) {
	sum, err := s.summary(ctx, a.SpaceID(), req)
	if err != nil {
		return nil, internalErr(s.log, ctx, "stats failed", err)
	}
	return sum, nil
}

// ownedSpace is the id of the space userID owns, or "" when they own none.
func (s *statsService) ownedSpace(ctx context.Context, userID string) (string, error) {
	m, err := s.memberships.ForUser(ctx, userID)
	if apperrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if m.Role != userrole.Owner {
		return "", nil
	}
	return m.SpaceID, nil
}

func (s *statsService) ProfileStats(ctx context.Context, userID string) (*bo.ProfileStats, error) {
	spaceID, err := s.ownedSpace(ctx, userID)
	if err != nil || spaceID == "" {
		return nil, err
	}
	sum, err := s.summary(ctx, spaceID, &request.RangeReq{})
	if err != nil {
		return nil, err
	}
	habits, err := s.entities.List(ctx, spaceID, entitytype.Habit, mongorepo.EntityQuery{})
	if err != nil {
		return nil, err
	}
	active := 0
	for i := range habits {
		if habitFromEntity(&habits[i]).Active {
			active++
		}
	}
	return &bo.ProfileStats{CurrentStreak: sum.CurrentStreak, LongestStreak: sum.LongestStreak, Habits: active}, nil
}

// activityDays is how far back a friend's activity graph goes: 53 weeks, as drawn by the app.
const activityDays = 53 * 7

func (s *statsService) ProfileActivity(ctx context.Context, userID string) (*bo.ProfileActivity, error) {
	spaceID, err := s.ownedSpace(ctx, userID)
	if err != nil || spaceID == "" {
		return nil, err
	}
	sc, err := s.score(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	today, _ := time.Parse(utils.DateLayout, sc.today)
	since := today.AddDate(0, 0, -activityDays).Format(utils.DateLayout)
	// Only the heat level leaves here: rest days, and so period days, look like any other day.
	out := &bo.ProfileActivity{Days: []bo.ActivityDay{}, TodayHabits: []bo.TodayHabit{}}
	for _, d := range sc.days {
		if d.Date <= since {
			continue
		}
		if lvl := scoring.HeatLevel(d.Percent, sc.settings.Threshold); lvl > 0 {
			out.Days = append(out.Days, bo.ActivityDay{Date: d.Date, Level: lvl})
		}
	}
	ents := append([]dao.Entity(nil), sc.habitEnts...)
	sort.SliceStable(ents, func(i, j int) bool {
		a, _ := ents[i].Data["sortOrder"].(float64)
		b, _ := ents[j].Data["sortOrder"].(float64)
		return a < b
	})
	logs := sc.byDate[sc.today]
	for i := range ents {
		h := habitFromEntity(&ents[i])
		if !h.Active || !scoring.IsScheduled(h, int(today.Weekday())) {
			continue
		}
		title, _ := ents[i].Data["title"].(string)
		icon, _ := ents[i].Data["icon"].(string)
		l, logged := logs[h.ID]
		out.TodayHabits = append(out.TodayHabits, bo.TodayHabit{Title: title, Icon: icon, Done: logged && scoring.Status(h, l) == scoring.Complete})
	}
	return out, nil
}

// spaceScore is a space's scored history from its first log through today, in the owner's timezone.
type spaceScore struct {
	today     string
	habitEnts []dao.Entity
	habits    []scoring.Habit
	byDate    map[string]map[string]scoring.Log
	settings  scoring.Settings
	days      []scoring.Day
}

func (s *statsService) score(ctx context.Context, spaceID string) (*spaceScore, error) {
	owner := s.spaceOwner(ctx, spaceID)
	tz := "UTC"
	if owner != nil {
		tz = owner.Timezone
	}
	today := scoring.LocalDate(s.clock.Now(), tz)
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
	// Sunday is a rest day unless the owner turned it off (absent = on).
	if v, ok := sundayRest(ctx, s.entities, spaceID); ok && v {
		settings.RestDays = appendUnique(settings.RestDays, 0)
	}
	// Period days are neutral only for an owner whose tracker is enabled. The reason is never exposed:
	// days[].restDay is the same flag as for Sundays.
	if owner.IsPeriodTrackerEnabled() {
		periods, err := s.entities.List(ctx, spaceID, entitytype.Period, mongorepo.EntityQuery{})
		if err != nil {
			return nil, err
		}
		settings.NeutralDates = periodNeutralDates(periods, today)
	}
	all, err := scoring.ComputeDays(habits, byDate, settings, earliest, today, today)
	if err != nil {
		return nil, err
	}
	return &spaceScore{today: today, habitEnts: habitEnts, habits: habits, byDate: byDate, settings: settings, days: all}, nil
}

func (s *statsService) summary(ctx context.Context, spaceID string, req *request.RangeReq) (*response.SummaryResponse, error) {
	sc, err := s.score(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	today, all := sc.today, sc.days
	from, to := req.From, req.To
	if to == "" {
		to = today
	}
	if from == "" {
		t, _ := time.Parse(utils.DateLayout, to)
		from = t.AddDate(0, 0, -29).Format(utils.DateLayout)
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

// sundayRest reads settings.sundayRest; an absent field (legacy settings or no settings record) means true.
func sundayRest(ctx context.Context, entities mongorepo.IEntityRepo, spaceID string) (value, ok bool) {
	se, err := entities.Get(ctx, spaceID, entitytype.Settings, constants.SettingsID)
	if err != nil {
		return true, true
	}
	if v, present := se.Data["sundayRest"].(bool); present {
		return v, true
	}
	return true, true
}

func appendUnique(xs []int, v int) []int {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}
