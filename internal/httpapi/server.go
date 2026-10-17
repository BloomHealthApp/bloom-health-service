// Package httpapi exposes the REST + sync API described in docs/api-contract.md.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/bubu-bloom/service/internal/auth"
	"github.com/bubu-bloom/service/internal/model"
	"github.com/bubu-bloom/service/internal/service"
	"github.com/bubu-bloom/service/internal/store"
)

const maxBody = 1 << 20

type Server struct {
	Auth       *auth.Service
	Svc        *service.Service
	Store      store.Store
	Log        *slog.Logger
	TrustProxy bool
	Version    string

	ipLimit    *limiters
	authLimit  *limiters
	emailLimit *limiters
}

func NewServer(a *auth.Service, svc *service.Service, st store.Store, log *slog.Logger, trustProxy bool) *Server {
	return &Server{Auth: a, Svc: svc, Store: st, Log: log, TrustProxy: trustProxy, Version: "1.0.0",
		ipLimit:    newLimiters(rate.Limit(30), 90),
		authLimit:  newLimiters(rate.Every(6*time.Second), 10),
		emailLimit: newLimiters(rate.Every(2*time.Minute), 3)}
}

// ---- rate limiting ----

type limiters struct {
	mu   sync.Mutex
	m    map[string]*entry
	r    rate.Limit
	b    int
	last time.Time
}
type entry struct {
	l    *rate.Limiter
	seen time.Time
}

func newLimiters(r rate.Limit, b int) *limiters {
	return &limiters{m: map[string]*entry{}, r: r, b: b}
}

func (ls *limiters) allow(key string) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	now := time.Now()
	if now.Sub(ls.last) > 5*time.Minute {
		for k, e := range ls.m {
			if now.Sub(e.seen) > 30*time.Minute {
				delete(ls.m, k)
			}
		}
		ls.last = now
	}
	e, ok := ls.m[key]
	if !ok {
		e = &entry{l: rate.NewLimiter(ls.r, ls.b)}
		ls.m[key] = e
	}
	e.seen = now
	return e.l.Allow()
}

func (s *Server) clientIP(r *http.Request) string {
	if s.TrustProxy {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			return strings.TrimSpace(strings.Split(xf, ",")[0])
		}
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// ---- JSON helpers ----

type errBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string, details map[string]any) {
	var b errBody
	b.Error.Code, b.Error.Message, b.Error.Details = code, msg, details
	writeJSON(w, status, b)
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	s.Log.Error("internal error", "err", err.Error(), "route", routePattern(r), "requestId", middleware.GetReqID(r.Context()))
	writeErr(w, http.StatusInternalServerError, "internal", "something went wrong", nil)
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

// decode reads a strict JSON body (unknown fields rejected, size limited).
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_failed", "invalid JSON body: "+shortErr(err), nil)
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "validation_failed", "unexpected data after JSON body", nil)
		return false
	}
	return true
}

func shortErr(err error) string {
	m := err.Error()
	if len(m) > 120 {
		m = m[:120]
	}
	return m
}

// ---- middleware ----

type ctxKey int

const actorKey ctxKey = 1

func actorFrom(r *http.Request) service.Actor { return r.Context().Value(actorKey).(service.Actor) }

func (s *Server) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		s.Log.Info("request", "method", r.Method, "route", routePattern(r), "status", ww.Status(),
			"ms", time.Since(start).Milliseconds(), "requestId", middleware.GetReqID(r.Context()))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.Log.Error("panic", "value", rec, "route", routePattern(r))
				writeErr(w, http.StatusInternalServerError, "internal", "something went wrong", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.ipLimit.allow(s.clientIP(r)) {
			w.Header().Set("Retry-After", "5")
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authLimit.allow(s.clientIP(r)) {
			w.Header().Set("Retry-After", "30")
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate verifies the bearer token and resolves membership from the database.
// Roles are never read from the client.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "missing bearer token", nil)
			return
		}
		uid, err := s.Auth.ParseAccess(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			if errors.Is(err, auth.ErrExpired) {
				writeErr(w, http.StatusUnauthorized, "token_expired", "access token expired", nil)
				return
			}
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid token", nil)
			return
		}
		u, err := s.Store.UserByID(r.Context(), uid)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid token", nil)
			return
		}
		m, err := s.Store.MembershipForUser(r.Context(), uid)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeErr(w, http.StatusForbidden, "forbidden", "not a member of a shared space", nil)
				return
			}
			s.internal(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey, service.Actor{User: u, Membership: m})))
	})
}

func (s *Server) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !actorFrom(r).IsOwner() {
			writeErr(w, http.StatusForbidden, "forbidden", "read-only access", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, s.recoverer, s.secureHeaders, s.logRequests, s.limitIP)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeErr(w, 404, "not_found", "no such route", nil) })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, 405, "validation_failed", "method not allowed", nil)
	})

	r.Route("/v1", func(r chi.Router) {
		r.Get("/health", s.health)
		r.With(s.limitAuth).Route("/auth", func(r chi.Router) {
			r.Post("/otp/request", s.otpRequest)
			r.Post("/otp/verify", s.otpVerify)
			r.Post("/refresh", s.refresh)
			r.Post("/logout", s.logout)
		})
		r.Group(func(r chi.Router) {
			r.Use(s.authenticate)
			r.Get("/me", s.getMe)
			r.Patch("/me", s.patchMe)
			r.Get("/settings", s.getSettings)
			r.Get("/habits", s.listHabits)
			r.Get("/logs", s.listLogs)
			r.Get("/workouts", s.listWorkouts)
			r.Get("/stats/summary", s.stats)
			r.Get("/sync/pull", s.syncPull)
			r.Group(func(r chi.Router) {
				r.Use(s.requireOwner)
				r.Patch("/settings", s.patchSettings)
				r.Post("/habits", s.createHabit)
				r.Patch("/habits/{id}", s.patchHabit)
				r.Delete("/habits/{id}", s.deleteHabit)
				r.Put("/logs/{habitId}/{localDate}", s.putLog)
				r.Post("/workouts", s.createWorkout)
				r.Patch("/workouts/{id}", s.patchWorkout)
				r.Delete("/workouts/{id}", s.deleteWorkout)
				r.Post("/sync/push", s.syncPush)
			})
		})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Ping(r.Context()); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "internal", "unavailable", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339), "version": s.Version})
}

var _ = model.RoleOwner
