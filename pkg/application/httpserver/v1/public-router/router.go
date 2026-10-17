// Package publicrouter holds the /v1 handlers described in docs/api-contract.md. Every handler follows
// the same shape: bind the DTO, read request-context values, Validate(), call the service, respond.
package publicrouter

import (
	"github.com/gin-gonic/gin"

	"github.com/bubu-bloom/service/pkg/middleware"
)

// RegisterRoutes mounts the public API on the /v1 group.
func RegisterRoutes(v1 *gin.RouterGroup) {
	v1.GET("/health", Health)

	auth := v1.Group("/auth", middleware.LimitAuth())
	auth.POST("/otp/request", OTPRequest)
	auth.POST("/otp/verify", OTPVerify)
	auth.POST("/refresh", Refresh)
	auth.POST("/logout", Logout)

	authed := v1.Group("", middleware.Authenticate())
	authed.GET("/me", GetMe)
	authed.PATCH("/me", PatchMe)
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
}
