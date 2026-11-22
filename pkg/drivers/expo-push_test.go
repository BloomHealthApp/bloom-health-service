package drivers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpoPushSendsAndReportsUnregisteredTokens(t *testing.T) {
	var got []PushMessage
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"data":[{"status":"ok","id":"a"},{"status":"error","message":"x","details":{"error":"DeviceNotRegistered"}}]}`))
	}))
	defer srv.Close()
	s := &expoPushSender{url: srv.URL, token: "tok", client: srv.Client()}
	res, err := s.Send(context.Background(), []PushMessage{{To: "ExponentPushToken[a]", Title: "Bloom", Body: "Riya sent you a message"}, {To: "ExponentPushToken[b]"}})
	require.NoError(t, err)
	require.Equal(t, "Bearer tok", auth)
	require.Len(t, got, 2)
	require.Equal(t, []string{"ExponentPushToken[b]"}, res.Unregistered)
}
