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
	options    *notifier.BroadcastOptions
	requests   []notifier.BroadcastRequest
	optionsErr error
	sendErr    error
}

func (f *fakeBroadcastControl) ListBroadcastOptions(context.Context) (*notifier.BroadcastOptions, error) {
	return f.options, f.optionsErr
}

func (f *fakeBroadcastControl) SendTestBroadcast(_ context.Context, request notifier.BroadcastRequest) error {
	f.requests = append(f.requests, request)
	return f.sendErr
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
