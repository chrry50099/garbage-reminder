package notifier

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoogleCloudTTSListVoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("languageCode"); got != "cmn-TW" {
			t.Fatalf("unexpected languageCode: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"voices": []map[string]interface{}{
				{
					"name":                   "cmn-TW-Wavenet-A",
					"languageCodes":          []string{"cmn-TW"},
					"ssmlGender":             "FEMALE",
					"naturalSampleRateHertz": 24000,
				},
			},
		})
	}))
	defer server.Close()

	provider := NewGoogleCloudTTS("test-key", t.TempDir())
	provider.baseURL = server.URL

	voices, err := provider.ListVoices(context.Background(), "cmn-TW")
	if err != nil {
		t.Fatalf("ListVoices() error: %v", err)
	}
	if len(voices) != 1 || voices[0].Name != "cmn-TW-Wavenet-A" {
		t.Fatalf("unexpected voices: %+v", voices)
	}
}

func TestGoogleCloudTTSSynthesizeToMediaSource(t *testing.T) {
	audio := base64.StdEncoding.EncodeToString([]byte("ID3"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/text:synthesize" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload googleCloudSynthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Voice.LanguageCode != "cmn-TW" {
			t.Fatalf("unexpected language code: %+v", payload)
		}
		if payload.AudioConfig.SpeakingRate != 1.1 {
			t.Fatalf("unexpected speaking rate: %+v", payload.AudioConfig)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"audioContent": audio})
	}))
	defer server.Close()

	mediaDir := t.TempDir()
	provider := NewGoogleCloudTTS("test-key", mediaDir)
	provider.baseURL = server.URL

	mediaSourceID, err := provider.SynthesizeToMediaSource(context.Background(), BroadcastRequest{
		Message:         "垃圾車快到了",
		TargetEntityIDs: []string{"media_player.ke_ting"},
		TTSEntityID:     googleCloudDirectEntityID,
		Language:        "cmn-TW",
		SpeakingRate:    1.1,
	})
	if err != nil {
		t.Fatalf("SynthesizeToMediaSource() error: %v", err)
	}
	if !strings.HasPrefix(mediaSourceID, "media-source://media_source/local/garbage_eta/generated/") {
		t.Fatalf("unexpected media source id: %s", mediaSourceID)
	}
	matches, err := filepath.Glob(filepath.Join(mediaDir, "*.mp3"))
	if err != nil {
		t.Fatalf("glob synthesized files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 synthesized file, got %d", len(matches))
	}
}
