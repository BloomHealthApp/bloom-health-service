package drivers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/config"
)

// ExpoPushURL is Expo's push service, which forwards to FCM (Android) and APNs (iOS).
const ExpoPushURL = "https://exp.host/--/api/v2/push/send"

// PushMessage is one notification to one device. Never put private text (messages, health data) in Title or Body:
// they show on the lock screen.
type PushMessage struct {
	To        string            `json:"to"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data,omitempty"`
	ChannelID string            `json:"channelId,omitempty"`
	Sound     string            `json:"sound,omitempty"`
}

// PushResult lists the tokens Expo says are no longer registered, so they can be deleted.
type PushResult struct {
	Unregistered []string
}

// IPushSender sends push notifications through Expo.
type IPushSender interface {
	Send(ctx context.Context, msgs []PushMessage) (*PushResult, error)
}

type expoPushSender struct {
	url    string
	token  string
	client *http.Client
}

// NewExpoPushSender sends with a short timeout, because it runs inside the request that triggered it.
func NewExpoPushSender(cfg *config.Store) IPushSender {
	return &expoPushSender{url: ExpoPushURL, token: cfg.ExpoAccessToken, client: &http.Client{Timeout: 3 * time.Second}}
}

type expoTicket struct {
	Status  string `json:"status"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

func (s *expoPushSender) Send(ctx context.Context, msgs []PushMessage) (*PushResult, error) {
	out := &PushResult{}
	if len(msgs) == 0 {
		return out, nil
	}
	body, err := json.Marshal(msgs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo push: status %d", resp.StatusCode)
	}
	var parsed struct {
		Data []expoTicket `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	// Tickets come back in the order the messages were sent.
	for i, t := range parsed.Data {
		if i < len(msgs) && t.Status == "error" && t.Details.Error == "DeviceNotRegistered" {
			out.Unregistered = append(out.Unregistered, msgs[i].To)
		}
	}
	return out, nil
}
