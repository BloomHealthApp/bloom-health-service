// Package constants groups values shared across layers.
package constants

import "time"

const (
	// RequestTimeout bounds each /v1 request, so a stuck database call fails fast with 503 instead of hanging until
	// the host kills it. 12s leaves room for a full 100-operation sync push and stays under the clients' 15s timeout.
	RequestTimeout = 12 * time.Second

	// ProfileLinkCodeLen is the length of the random base62 code in a profile link (about 71 bits).
	ProfileLinkCodeLen = 12
	// DeclinedRequestCooldown is how long a declined requester must wait before asking the same user again.
	DeclinedRequestCooldown = 7 * 24 * time.Hour
	// MaxConnectionsListed caps each list returned by GET /v1/connections.
	MaxConnectionsListed = 200

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
	// DeletedChatRetention is how long a deleted chat session's empty tombstone is kept before MongoDB's TTL removes it.
	DeletedChatRetention = 7 * 24 * time.Hour
	// AccountDeletionGrace is how long a deleted account stays locked and restorable before it is erased.
	AccountDeletionGrace = 15 * 24 * time.Hour
	// PurgeBatch is how many due accounts one purge pass loads at a time.
	PurgeBatch = 50

	MaxChatContentChars   = 4000
	MaxChatTitleChars     = 80
	DefaultChatTitle      = "New chat"
	DefaultChatPageLimit  = 30
	MaxChatPageLimit      = 50
	MaxChatSessionsListed = 100
	// MaxDMChars is the longest message between friends, in runes.
	MaxDMChars = 1000
	// DefaultDMPageLimit and MaxDMPageLimit bound GET /dm/{userId}/messages.
	DefaultDMPageLimit = 30
	MaxDMPageLimit     = 100
	// DMDailyCap is how many messages one user can send per UTC day.
	DMDailyCap = 500
	// MaxDMThreadsListed bounds the inbox.
	MaxDMThreadsListed = 200
	// MaxPushTokensPerUser keeps the newest devices only; older ones drop off.
	MaxPushTokensPerUser = 10
	// MaxPushTokenChars bounds a stored Expo push token.
	MaxPushTokenChars = 200
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
	// Chat collections are shared with bloom-health-task-worker (docs/chat-contract.md).
	CollChatSessions = "chatSessions"
	CollChatMessages = "chatMessages"
	CollConnections  = "connections"
	CollDMThreads    = "dmThreads"
	CollDMs          = "directMessages"
)
