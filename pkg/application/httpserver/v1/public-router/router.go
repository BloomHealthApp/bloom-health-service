// Package publicrouter holds the /v1 handlers described in docs/api-contract.md.
package publicrouter

import (
	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/middleware"
)

// RegisterRoutes mounts the public API on the /v1 group.
func RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/health", HandleHealth)

	auth := v1.Group("/auth", middleware.LimitAuth())
	auth.POST("/signup/request", HandleSignUpRequest)
	auth.POST("/signin/request", HandleSignInRequest)
	auth.POST("/otp/verify", HandleVerifyOTP)
	auth.POST("/refresh", HandleRefresh)
	auth.POST("/logout", HandleLogout)

	signedIn := v1.Group("", middleware.Authenticate())
	// While a deletion is pending only these work, so the app can show and restore the account.
	signedIn.GET("/me", HandleGetMe)
	signedIn.DELETE("/me", HandleDeleteMe)
	signedIn.POST("/me/restore", HandleRestoreMe)

	authed := signedIn.Group("", middleware.RequireActiveAccount())
	authed.PATCH("/me", HandlePatchMe)
	authed.POST("/me/phone/request", HandleRequestPhoneChange)
	authed.POST("/me/phone/verify", HandleVerifyPhoneChange)
	authed.GET("/settings", HandleGetSettings)
	authed.GET("/habits", HandleListHabits)
	authed.GET("/logs", HandleListLogs)
	authed.GET("/workouts", HandleListWorkouts)
	authed.GET("/stats/summary", HandleGetStatsSummary)
	authed.GET("/sync/pull", HandleSyncPull)
	// AI chat: any role, every session is private to the calling user
	authed.GET("/chat/models", HandleListChatModels)
	authed.POST("/chat/sessions", HandleCreateChatSession)
	authed.GET("/chat/sessions", HandleListChatSessions)
	authed.GET("/chat/sessions/:sid", HandleGetChatSession)
	authed.DELETE("/chat/sessions/:sid", HandleDeleteChatSession)
	authed.GET("/chat/sessions/:sid/messages", HandleListChatMessages)
	authed.POST("/chat/sessions/:sid/messages", HandleSendChatMessage)

	owner := authed.Group("", middleware.RequireOwner())
	owner.PATCH("/settings", HandlePatchSettings)
	owner.POST("/habits", HandleCreateHabit)
	owner.PATCH("/habits/:id", HandlePatchHabit)
	owner.DELETE("/habits/:id", HandleDeleteHabit)
	owner.PUT("/logs/:habitId/:localDate", HandlePutLog)
	owner.POST("/workouts", HandleCreateWorkout)
	owner.PATCH("/workouts/:id", HandlePatchWorkout)
	owner.DELETE("/workouts/:id", HandleDeleteWorkout)
	owner.POST("/sync/push", HandleSyncPush)
	// period tracker: owner only (viewers get 403) and only while the owner's tracker is enabled
	owner.GET("/periods", HandleListPeriods)
	owner.POST("/periods", HandleCreatePeriod)
	owner.PATCH("/periods/:id", HandlePatchPeriod)
	owner.DELETE("/periods/:id", HandleDeletePeriod)
}
