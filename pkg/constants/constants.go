// Package constants groups values shared across layers.
package constants

import "time"

const (
	// MaxBodyBytes is the request body limit (1 MiB).
	MaxBodyBytes = 1 << 20

	SettingsID = "settings"

	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
	OTPTTL          = 10 * time.Minute
	OTPMaxAttempts  = 5

	MaxSyncOperations = 100
	MaxRangeDays      = 366
	DefaultPullLimit  = 500
	MaxPullLimit      = 1000
	OpRetention       = 60 * 24 * time.Hour
)

// Request header names understood by the generic-request-info middleware.
const (
	HeaderAuthorization = "Authorization"
	HeaderRequestID     = "X-Request-Id"
	HeaderDeviceID      = "X-Device-Id"
	HeaderClientType    = "X-Client-Type"
	HeaderAppVersion    = "X-App-Version"
	HeaderTimezone      = "X-Timezone"
)

// Environment names selected by ACTIVE_ENV.
const (
	EnvDev  = "DEV"
	EnvStag = "STAG"
	EnvProd = "PROD"
)

// Store kinds.
const (
	StoreMongo  = "mongo"
	StoreMemory = "memory"
)

// Mongo collection names.
const (
	CollUsers       = "users"
	CollSpaces      = "sharedSpaces"
	CollMemberships = "memberships"
	CollHabits      = "habits"
	CollDailyLogs   = "dailyHabitLogs"
	CollWorkouts    = "workoutSessions"
	CollSettings    = "settings"
	CollPeriods     = "periods"
	CollOps         = "appliedOperations"
	CollOTP         = "otpChallenges"
	CollSessions    = "refreshSessions"
	CollCounters    = "counters"
)
