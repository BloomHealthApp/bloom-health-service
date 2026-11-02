// Documented MongoDB index set for Bloom Health. The service creates the same indexes at startup
// (pkg/infrastructure/mongorepo/common.go: EnsureIndexes); run this script manually to provision a
// database up front, e.g. when the application user has no createIndex privilege:
//
//   mongosh "$MONGODB_URI" --file ddl/mongodb/indexes.js
//
// Collections: users, memberships, sharedSpaces, habits, dailyHabitLogs, workoutSessions, settings, periods,
// appliedOperations, otpChallenges, refreshSessions, counters (per-space sync sequence), chatSessions, chatMessages.
// Writes use single-document compare-and-swap on `version`; no multi-document transactions are needed.

const dbName = process.env.MONGODB_DB || "bubu_bloom";
const d = db.getSiblingDB(dbName);

// One user per lower-cased email.
d.users.createIndex({ emailLower: 1 }, { unique: true });
// One user per E.164 phone; partial so legacy users without a phone field do not collide.
d.users.createIndex({ phone: 1 }, { unique: true, partialFilterExpression: { phone: { $type: "string" } } });

// One membership per user per space; lookup by user on every authenticated request.
d.memberships.createIndex({ sharedSpaceId: 1, userId: 1 }, { unique: true });
d.memberships.createIndex({ userId: 1 });

// Entity collections: sync pull cursor is (spaceId, seq).
["habits", "dailyHabitLogs", "workoutSessions", "settings", "periods"].forEach((c) => {
  d[c].createIndex({ spaceId: 1, seq: 1 });
});
// Daily logs are unique per habit and local date within a space.
d.dailyHabitLogs.createIndex({ spaceId: 1, "data.habitId": 1, "data.localDate": 1 }, { unique: true });
d.dailyHabitLogs.createIndex({ spaceId: 1, "data.localDate": 1 });
d.workoutSessions.createIndex({ spaceId: 1, "data.localDate": 1 });
// Periods (private to the space owner; filtered out of viewer reads in the service layer).
d.periods.createIndex({ spaceId: 1, "data.startDate": 1 });

// Idempotency records for sync operations, retained 60 days (>= 30 days required by the contract).
d.appliedOperations.createIndex({ createdAt: 1 }, { expireAfterSeconds: 60 * 60 * 24 * 60 });

// One-time codes expire at expiresAt; refresh sessions are hashed, unique and expire at expiresAt.
// Challenges are keyed by challengeId (_id); the (email, createdAt) index serves the resend cooldown lookup.
d.otpChallenges.createIndex({ expiresAt: 1 }, { expireAfterSeconds: 0 });
d.otpChallenges.createIndex({ email: 1, createdAt: 1 });
d.refreshSessions.createIndex({ tokenHash: 1 }, { unique: true });
d.refreshSessions.createIndex({ familyId: 1 });
d.refreshSessions.createIndex({ expiresAt: 1 }, { expireAfterSeconds: 0 });

// AI chat (shared with bloom-health-task-worker, see docs/chat-contract.md): sessions listed per user by
// recency; messages unique per (sessionId, idx); (status, runAt, createdAt) serves the worker's claim
// query; (userId, role, createdAt) serves the daily message cap.
d.chatSessions.createIndex({ userId: 1, updatedAt: -1 });
d.chatMessages.createIndex({ sessionId: 1, idx: 1 }, { unique: true });
d.chatMessages.createIndex({ status: 1, runAt: 1, createdAt: 1 });
d.chatMessages.createIndex({ userId: 1, role: 1, createdAt: 1 });
