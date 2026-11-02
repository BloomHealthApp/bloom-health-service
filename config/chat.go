package config

import (
	"errors"
	"fmt"
)

// ChatModel is one entry of the chat model catalogue. bloom-health-task-worker carries the same list
// and routes on Vendor; this service only validates ids and lists the models.
type ChatModel struct {
	ID              string `mapstructure:"id"`
	Label           string `mapstructure:"label"`
	Vendor          string `mapstructure:"vendor"`
	ProviderModelID string `mapstructure:"providerModelId"`
	ContextWindow   int    `mapstructure:"contextWindow"`
	Default         bool   `mapstructure:"default"`
}

// ChatConfig is the "chat" section of the config file.
type ChatConfig struct {
	Models []ChatModel `mapstructure:"models"`
	// DailyMessageCap is the number of messages a user may send per UTC day (protects the free model quota).
	DailyMessageCap int `mapstructure:"dailyMessageCap"`
}

// DefaultChatModels is the v1 catalogue: one free Groq model.
func DefaultChatModels() []ChatModel {
	return []ChatModel{{ID: "groq-gpt-oss-120b", Label: "GPT-OSS 120B", Vendor: "groq",
		ProviderModelID: "openai/gpt-oss-120b", ContextWindow: 131072, Default: true}}
}

// WithDefaults fills unset values, so a zero ChatConfig (tests) behaves like the shipped config.
func (c ChatConfig) WithDefaults() ChatConfig {
	if len(c.Models) == 0 {
		c.Models = DefaultChatModels()
	}
	if c.DailyMessageCap <= 0 {
		c.DailyMessageCap = 100
	}
	return c
}

// Validate requires unique model ids and exactly one default model.
func (c ChatConfig) Validate() error {
	if len(c.Models) == 0 {
		return nil
	}
	seen := map[string]bool{}
	defaults := 0
	for _, m := range c.Models {
		if m.ID == "" || m.Vendor == "" || m.ProviderModelID == "" {
			return errors.New("chat.models entries need id, vendor and providerModelId")
		}
		if seen[m.ID] {
			return fmt.Errorf("chat.models has duplicate id %q", m.ID)
		}
		seen[m.ID] = true
		if m.Default {
			defaults++
		}
	}
	if defaults != 1 {
		return errors.New("chat.models needs exactly one default model")
	}
	return nil
}

// Model returns the catalogue entry with id.
func (c ChatConfig) Model(id string) (ChatModel, bool) {
	for _, m := range c.Models {
		if m.ID == id {
			return m, true
		}
	}
	return ChatModel{}, false
}

// DefaultModel returns the model used when a request names none.
func (c ChatConfig) DefaultModel() ChatModel {
	for _, m := range c.Models {
		if m.Default {
			return m
		}
	}
	return c.Models[0]
}
