// Package publicrouter holds the /v1 handlers described in docs/api-contract.md.
package publicrouter

import (
	"github.com/gin-gonic/gin"

	"github.com/BloomHealthApp/bloom-health-service/pkg/middleware"
)

// RegisterRoutes mounts the public API on the /v1 group.
func RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/health", Health)

	auth := v1.Group("/auth", middleware.LimitAuth())
	auth.POST("/signup/request", SignUpRequest)
	auth.POST("/signin/request", SignInRequest)
	auth.POST("/otp/verify", OTPVerify)
	auth.POST("/refresh", Refresh)
	auth.POST("/logout", Logout)

	authed := v1.Group("", middleware.Authenticate())
	authed.GET("/me", GetMe)
	authed.PATCH("/me", PatchMe)
	authed.POST("/me/phone/request", RequestPhoneChange)
	authed.POST("/me/phone/verify", VerifyPhoneChange)
	authed.GET("/settings", GetSettings)
	authed.GET("/habits", ListHabits)
	authed.GET("/logs", ListLogs)
	authed.GET("/workouts", ListWorkouts)
	authed.GET("/stats/summary", StatsSummary)
	authed.GET("/sync/pull", SyncPull)

	owner := authed.Group("", middleware.RequireOwner())
	owner.PATCH("/settings", PatchSettings)
	owner.POST("/habits", CreateHabit)
	owner.PATCH("/habits/:id", PatchHabit)
	owner.DELETE("/habits/:id", DeleteHabit)
	owner.PUT("/logs/:habitId/:localDate", PutLog)
	owner.POST("/workouts", CreateWorkout)
	owner.PATCH("/workouts/:id", PatchWorkout)
	owner.DELETE("/workouts/:id", DeleteWorkout)
	owner.POST("/sync/push", SyncPush)
	// period tracker: owner only (viewers get 403) and only while the owner's tracker is enabled
	owner.GET("/periods", ListPeriods)
	owner.POST("/periods", CreatePeriod)
	owner.PATCH("/periods/:id", PatchPeriod)
	owner.DELETE("/periods/:id", DeletePeriod)
}
