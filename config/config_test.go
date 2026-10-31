package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigCORSOrigins(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{"empty disables CORS", "", nil},
		{"single origin", "https://app.example.com", []string{"https://app.example.com"}},
		{"list is trimmed", " https://a.example.com, https://b.example.com ,", []string{"https://a.example.com", "https://b.example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ACTIVE_ENV", "DEV")
			t.Setenv("STORE", "memory")
			t.Setenv("MAILER", "console")
			t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-test-secret-test-secret")
			t.Setenv("CORS_ORIGINS", tt.env)

			cfg, err := NewConfig()

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.CORSOrigins)
		})
	}
}
