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
	assert.Equal(t, ChatModel{ID: "groq-gpt-oss-120b", Label: "GPT-OSS 120B", Vendor: "groq",
		ProviderModelID: "openai/gpt-oss-120b", ContextWindow: 131072, Default: true}, cfg.Chat.Models[0])

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

func TestValidateReviewLogin(t *testing.T) {
	assert.NoError(t, validateReviewLogin("", ""), "off by default")
	assert.NoError(t, validateReviewLogin("review@example.com", "481937"))
	assert.Error(t, validateReviewLogin("review@example.com", ""), "both or neither")
	assert.Error(t, validateReviewLogin("", "481937"), "both or neither")
	assert.Error(t, validateReviewLogin("not-an-email", "481937"))
	for _, code := range []string{"12345", "1234567", "12a456", "123456", "654321", "000000", "777777"} {
		assert.Error(t, validateReviewLogin("review@example.com", code), code)
	}
}

func TestNewConfigReviewLogin(t *testing.T) {
	t.Setenv("ACTIVE_ENV", "DEV")
	t.Setenv("STORE", "memory")
	t.Setenv("MAILER", "console")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-test-secret-test-secret")
	t.Setenv("REVIEW_LOGIN_EMAIL", " Review@Example.com ")
	t.Setenv("REVIEW_LOGIN_CODE", "481937")

	cfg, err := NewConfig()
	require.NoError(t, err)
	assert.Equal(t, "review@example.com", cfg.ReviewEmail)
	assert.Equal(t, "481937", cfg.ReviewCode)

	for _, bad := range []string{"", "123456", `"481937"`, "48193 7"} {
		t.Setenv("REVIEW_LOGIN_CODE", bad)
		cfg, err = NewConfig()
		require.NoError(t, err, "a bad review login must not stop the service: %q", bad)
		assert.Empty(t, cfg.ReviewEmail, bad)
		assert.Empty(t, cfg.ReviewCode, bad)
		assert.NotEmpty(t, cfg.ReviewLoginProblem, bad)
	}
}
