package httpapi_test

import (
	"time"

	"github.com/google/uuid"

	"github.com/bubu-bloom/service/internal/model"
)

func newUser(email string) *model.User {
	now := time.Now().UTC()
	return &model.User{ID: uuid.NewString(), Email: email, DisplayName: "Outsider", Timezone: "UTC", Theme: "cottonCandy", CreatedAt: now, UpdatedAt: now}
}
