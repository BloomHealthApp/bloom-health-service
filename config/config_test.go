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

func TestNewConfigChat(t *testing.T) {
	t.Setenv("ACTIVE_ENV", "DEV")
	t.Setenv("STORE", "memory")
	t.Setenv("MAILER", "console")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-test-secret-test-secret")

	cfg, err := NewConfig()
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.Chat.DailyMessageCap)
	require.Len(t, cfg.Chat.Models, 1)
	assert.Equal(t, ChatModel{ID: "groq-llama-3.3-70b", Label: "Llama 3.3 70B", Vendor: "groq",
		ProviderModelID: "llama-3.3-70b-versatile", ContextWindow: 128000, Default: true}, cfg.Chat.Models[0])

	t.Setenv("CHAT_DAILY_MESSAGE_CAP", "7")
	cfg, err = NewConfig()
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Chat.DailyMessageCap)
}

func TestChatConfigValidate(t *testing.T) {
	m := DefaultChatModels()[0]
	other := m
	other.ID, other.Default = "other", false
	assert.NoError(t, ChatConfig{Models: []ChatModel{m, other}}.Validate())
	assert.Error(t, ChatConfig{Models: []ChatModel{m, m}}.Validate(), "duplicate id")
	other.Default = true
	assert.Error(t, ChatConfig{Models: []ChatModel{m, other}}.Validate(), "two defaults")
	assert.Error(t, ChatConfig{Models: []ChatModel{{ID: "x", Default: true}}}.Validate(), "missing vendor")
}
