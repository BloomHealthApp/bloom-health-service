package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	"github.com/BloomHealthApp/bloom-health-service/pkg/enums/userrole"
)

func TestUserResponse(t *testing.T) {
	ownerAvatar := "fox"
	u := &dao.User{ID: "u1", DisplayName: "Asha"}
	owner := &dao.User{ID: "u0", DisplayName: "Ravi", Avatar: &ownerAvatar}
	m := &dao.Membership{SpaceID: "sp1", Role: userrole.Viewer}
	tests := []struct {
		name      string
		owner     *dao.User
		wantName  string
		wantAvatr *string
	}{
		{"owner found", owner, "Ravi", &ownerAvatar},
		{"owner missing falls back to user", nil, "Asha", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UserResponse(u, m, tt.owner)
			assert.Equal(t, tt.wantName, got.OwnerDisplayName)
			assert.Equal(t, tt.wantAvatr, got.OwnerAvatar)
			assert.Equal(t, "sp1", got.SharedSpaceID)
			assert.Equal(t, string(userrole.Viewer), got.Role)
		})
	}
}
