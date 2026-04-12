package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListBroadcastOptionsFiltersHomePodsAndTTS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/states" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"entity_id": "media_player.ke_ting",
				"state":     "idle",
				"attributes": map[string]interface{}{
					"friendly_name": "客廳podhomepodmini播放器",
				},
			},
			{
				"entity_id": "media_player.living_room_tv",
				"state":     "idle",
				"attributes": map[string]interface{}{
					"friendly_name": "客廳電視",
				},
			},
			{
				"entity_id": "tts.google_en_com",
				"state":     "unknown",
				"attributes": map[string]interface{}{
					"friendly_name": "Google Translate",
				},
			},
		})
	}))
	defer server.Close()

	client := NewHomeAssistant(server.URL, "token", "webhook", "garbage")
	options, err := client.ListBroadcastOptions(context.Background())
	if err != nil {
		t.Fatalf("ListBroadcastOptions() error: %v", err)
	}

	if len(options.MediaPlayers) != 1 || options.MediaPlayers[0].EntityID != "media_player.ke_ting" {
		t.Fatalf("unexpected media players: %+v", options.MediaPlayers)
	}
	if len(options.TTSEntities) != 1 || options.DefaultTTSEntity != "tts.google_en_com" {
		t.Fatalf("unexpected tts entities: %+v", options)
	}
}

func TestSendTestBroadcastUsesTTSSpeakService(t *testing.T) {
	var requests []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/tts/speak" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHomeAssistant(server.URL, "token", "webhook", "garbage")
	err := client.SendTestBroadcast(context.Background(), BroadcastRequest{
		Message:         "test",
		TTSEntityID:     "tts.google_en_com",
		TargetEntityIDs: []string{"media_player.zhu_wo", "media_player.ke_ting"},
		Language:        "en",
	})
	if err != nil {
		t.Fatalf("SendTestBroadcast() error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if requests[0]["entity_id"] != "tts.google_en_com" || requests[0]["media_player_entity_id"] != "media_player.zhu_wo" {
		t.Fatalf("unexpected first payload: %+v", requests[0])
	}
	if requests[1]["media_player_entity_id"] != "media_player.ke_ting" {
		t.Fatalf("unexpected second payload: %+v", requests[1])
	}
}
