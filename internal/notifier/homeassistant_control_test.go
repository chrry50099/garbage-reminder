package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
					"friendly_name": "homepod mini",
				},
			},
			{
				"entity_id": "media_player.living_room_tv",
				"state":     "idle",
				"attributes": map[string]interface{}{
					"friendly_name": "living room tv",
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

func TestSendTestBroadcastGeneratesMediaAndPlaysIt(t *testing.T) {
	var playRequests []map[string]interface{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tts_get_url":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"url":  server.URL + "/api/tts_proxy/test.mp3",
				"path": "/api/tts_proxy/test.mp3",
			})
		case "/api/tts_proxy/test.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3"))
		case "/api/services/media_player/play_media":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			playRequests = append(playRequests, payload)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
	if len(playRequests) != 2 {
		t.Fatalf("expected 2 play_media requests, got %d", len(playRequests))
	}
	if playRequests[0]["entity_id"] != "media_player.zhu_wo" || !strings.Contains(playRequests[0]["media_content_id"].(string), "/api/tts_proxy/test.mp3") {
		t.Fatalf("unexpected first payload: %+v", playRequests[0])
	}
	if playRequests[1]["entity_id"] != "media_player.ke_ting" {
		t.Fatalf("unexpected second payload: %+v", playRequests[1])
	}
}

func TestSendTestBroadcastOmitsLanguageForGeminiTTS(t *testing.T) {
	var payload ttsGetURLRequest
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tts_get_url":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"url":  server.URL + "/api/tts_proxy/test.mp3",
				"path": "/api/tts_proxy/test.mp3",
			})
		case "/api/tts_proxy/test.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3"))
		case "/api/services/media_player/play_media":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHomeAssistant(server.URL, "token", "webhook", "garbage")
	err := client.SendTestBroadcast(context.Background(), BroadcastRequest{
		Message:         "test gemini voice",
		TTSEntityID:     "tts.google_ai_tts",
		TargetEntityIDs: []string{"media_player.ke_ting"},
		Language:        "zh-TW",
	})
	if err != nil {
		t.Fatalf("SendTestBroadcast() error: %v", err)
	}
	if payload.Language != "" {
		t.Fatalf("expected language to be omitted for Gemini TTS, got %+v", payload)
	}
	if payload.Options["voice"] != "achernar" {
		t.Fatalf("expected default Gemini voice achernar, got %+v", payload)
	}
}

func TestSendTestBroadcastFallsBackWhenPrimaryTTSProxyFails(t *testing.T) {
	var requestedEngines []string
	var played bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/states":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"entity_id": "tts.google_ai_tts",
					"state":     "unknown",
					"attributes": map[string]interface{}{
						"friendly_name": "Google AI TTS",
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
		case "/api/tts_get_url":
			var payload ttsGetURLRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requestedEngines = append(requestedEngines, payload.EngineID)
			if payload.EngineID == "tts.google_ai_tts" {
				_ = json.NewEncoder(w).Encode(map[string]string{
					"url":  server.URL + "/api/tts_proxy/broken.mp3",
					"path": "/api/tts_proxy/broken.mp3",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"url":  server.URL + "/api/tts_proxy/ok.mp3",
				"path": "/api/tts_proxy/ok.mp3",
			})
		case "/api/tts_proxy/broken.mp3":
			w.WriteHeader(http.StatusInternalServerError)
		case "/api/tts_proxy/ok.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3"))
		case "/api/services/media_player/play_media":
			played = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHomeAssistant(server.URL, "token", "webhook", "garbage")
	err := client.SendTestBroadcast(context.Background(), BroadcastRequest{
		Message:         "test",
		TTSEntityID:     "tts.google_ai_tts",
		TargetEntityIDs: []string{"media_player.zhu_wo"},
	})
	if err != nil {
		t.Fatalf("SendTestBroadcast() error: %v", err)
	}
	if !played {
		t.Fatalf("expected playback request after fallback")
	}
	if len(requestedEngines) < 2 || requestedEngines[0] != "tts.google_ai_tts" || requestedEngines[1] != "tts.google_en_com" {
		t.Fatalf("unexpected engine order: %+v", requestedEngines)
	}
}
