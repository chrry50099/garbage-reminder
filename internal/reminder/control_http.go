package reminder

import (
	"context"
	"encoding/json"
	"net/http"

	"telegram-garbage-reminder/internal/notifier"
)

type broadcastControl interface {
	ListBroadcastOptions(ctx context.Context) (*notifier.BroadcastOptions, error)
	SendTestBroadcast(ctx context.Context, request notifier.BroadcastRequest) error
	GetAutoBroadcastSettings(ctx context.Context) (*notifier.AutomaticBroadcastSettings, error)
	SaveAutoBroadcastSettings(ctx context.Context, settings notifier.AutomaticBroadcastSettings) (*notifier.AutomaticBroadcastSettings, error)
}

func NewBroadcastOptionsHandler(control broadcastControl) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if control == nil {
			http.Error(w, "home assistant control unavailable", http.StatusServiceUnavailable)
			return
		}

		options, err := control.ListBroadcastOptions(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(options)
	})
}

func NewBroadcastTestHandler(control broadcastControl) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if control == nil {
			http.Error(w, "home assistant control unavailable", http.StatusServiceUnavailable)
			return
		}

		var request notifier.BroadcastRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := control.SendTestBroadcast(r.Context(), request); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":                true,
			"target_entity_ids": request.TargetEntityIDs,
			"tts_entity_id":     request.TTSEntityID,
		})
	})
}

func NewAutoBroadcastSettingsHandler(control broadcastControl) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if control == nil {
			http.Error(w, "home assistant control unavailable", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			settings, err := control.GetAutoBroadcastSettings(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(settings)
		case http.MethodPost:
			var request notifier.AutomaticBroadcastSettings
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			settings, err := control.SaveAutoBroadcastSettings(r.Context(), request)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(settings)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
