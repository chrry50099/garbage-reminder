package reminder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"telegram-garbage-reminder/internal/notifier"
)

type fakeBroadcastControl struct {
	options      *notifier.BroadcastOptions
	requests     []notifier.BroadcastRequest
	autoSettings *notifier.AutomaticBroadcastSettings
	optionsErr   error
	sendErr      error
	autoErr      error
}

func (f *fakeBroadcastControl) ListBroadcastOptions(context.Context) (*notifier.BroadcastOptions, error) {
	return f.options, f.optionsErr
}

func (f *fakeBroadcastControl) SendTestBroadcast(_ context.Context, request notifier.BroadcastRequest) error {
	f.requests = append(f.requests, request)
	return f.sendErr
}

func (f *fakeBroadcastControl) GetAutoBroadcastSettings(context.Context) (*notifier.AutomaticBroadcastSettings, error) {
	return f.autoSettings, f.autoErr
}

func (f *fakeBroadcastControl) SaveAutoBroadcastSettings(_ context.Context, settings notifier.AutomaticBroadcastSettings) (*notifier.AutomaticBroadcastSettings, error) {
	if f.autoErr != nil {
		return nil, f.autoErr
	}
	copy := settings
	copy.TargetEntityIDs = append([]string(nil), settings.TargetEntityIDs...)
	f.autoSettings = &copy
	return f.autoSettings, nil
}

func TestBroadcastOptionsHandlerReturnsJSON(t *testing.T) {
	handler := NewBroadcastOptionsHandler(&fakeBroadcastControl{
		options: &notifier.BroadcastOptions{
			MediaPlayers: []notifier.BroadcastEntityOption{
				{EntityID: "media_player.ke_ting", FriendlyName: "客廳 HomePod"},
			},
			TTSEntities: []notifier.BroadcastEntityOption{
				{EntityID: "tts.google_en_com", FriendlyName: "Google Translate"},
			},
			DefaultTTSEntity: "tts.google_en_com",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/broadcast/options", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var payload notifier.BroadcastOptions
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.DefaultTTSEntity != "tts.google_en_com" || len(payload.MediaPlayers) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestBroadcastTestHandlerPassesRequest(t *testing.T) {
	control := &fakeBroadcastControl{}
	handler := NewBroadcastTestHandler(control)

	req := httptest.NewRequest(http.MethodPost, "/api/broadcast/test", strings.NewReader(`{"message":"test","target_entity_ids":["media_player.zhu_wo"],"tts_entity_id":"tts.google_en_com","language":"en"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if len(control.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(control.requests))
	}
	if control.requests[0].Message != "test" || control.requests[0].TargetEntityIDs[0] != "media_player.zhu_wo" {
		t.Fatalf("unexpected request: %+v", control.requests[0])
	}
}

func TestAutoBroadcastSettingsHandlerGetAndPost(t *testing.T) {
	control := &fakeBroadcastControl{
		autoSettings: &notifier.AutomaticBroadcastSettings{
			TargetEntityIDs: []string{"media_player.ke_ting"},
			TTSEntityID:     "tts.google_ai_tts",
			Voice:           "achernar",
		},
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/broadcast/auto", nil)
	getRecorder := httptest.NewRecorder()
	NewAutoBroadcastSettingsHandler(control).ServeHTTP(getRecorder, getReq)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET, got %d", getRecorder.Code)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/broadcast/auto", strings.NewReader(`{"target_entity_ids":["media_player.zhu_wo"],"tts_entity_id":"tts.google_en_com","language":"en"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postRecorder := httptest.NewRecorder()
	NewAutoBroadcastSettingsHandler(control).ServeHTTP(postRecorder, postReq)
	if postRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for POST, got %d", postRecorder.Code)
	}
	if control.autoSettings == nil || control.autoSettings.TTSEntityID != "tts.google_en_com" || len(control.autoSettings.TargetEntityIDs) != 1 {
		t.Fatalf("unexpected auto settings: %+v", control.autoSettings)
	}
}
