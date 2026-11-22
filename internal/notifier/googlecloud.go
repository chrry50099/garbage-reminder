package notifier

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const googleCloudDirectEntityID = "tts.google_cloud_tts_direct"

type GoogleCloudTTS struct {
	apiKey            string
	baseURL           string
	mediaDir          string
	mediaSourcePrefix string
	httpClient        *http.Client
}

type GoogleCloudVoiceOption struct {
	Name                   string   `json:"name"`
	Tier                   string   `json:"tier,omitempty"`
	LanguageCodes          []string `json:"language_codes,omitempty"`
	SSMLGender             string   `json:"ssml_gender,omitempty"`
	NaturalSampleRateHertz int      `json:"natural_sample_rate_hertz,omitempty"`
}

type googleCloudVoicesResponse struct {
	Voices []struct {
		LanguageCodes          []string `json:"languageCodes"`
		Name                   string   `json:"name"`
		SSMLGender             string   `json:"ssmlGender"`
		NaturalSampleRateHertz int      `json:"naturalSampleRateHertz"`
	} `json:"voices"`
}

type googleCloudSynthesizeRequest struct {
	Input       googleCloudSynthesizeInput       `json:"input"`
	Voice       googleCloudSynthesizeVoice       `json:"voice"`
	AudioConfig googleCloudSynthesizeAudioConfig `json:"audioConfig"`
}

type googleCloudSynthesizeInput struct {
	Text string `json:"text,omitempty"`
	SSML string `json:"ssml,omitempty"`
}

type googleCloudSynthesizeVoice struct {
	LanguageCode string `json:"languageCode,omitempty"`
	Name         string `json:"name,omitempty"`
}

type googleCloudSynthesizeAudioConfig struct {
	AudioEncoding   string   `json:"audioEncoding"`
	SpeakingRate    float64  `json:"speakingRate,omitempty"`
	Pitch           float64  `json:"pitch,omitempty"`
	VolumeGainDb    float64  `json:"volumeGainDb,omitempty"`
	EffectsProfileID []string `json:"effectsProfileId,omitempty"`
}

type googleCloudSynthesizeResponse struct {
	AudioContent string `json:"audioContent"`
}

func NewGoogleCloudTTS(apiKey, mediaDir string) *GoogleCloudTTS {
	return &GoogleCloudTTS{
		apiKey:            strings.TrimSpace(apiKey),
		baseURL:           "https://texttospeech.googleapis.com/v1",
		mediaDir:          strings.TrimSpace(mediaDir),
		mediaSourcePrefix: "media-source://media_source/local/garbage_eta/generated/",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (g *GoogleCloudTTS) Enabled() bool {
	return g != nil && strings.TrimSpace(g.apiKey) != "" && strings.TrimSpace(g.mediaDir) != ""
}

func (g *GoogleCloudTTS) ListVoices(ctx context.Context, languageCode string) ([]GoogleCloudVoiceOption, error) {
	if !g.Enabled() {
		return nil, fmt.Errorf("google cloud tts is not configured")
	}

	endpoint, err := url.Parse(strings.TrimRight(g.baseURL, "/") + "/voices")
	if err != nil {
		return nil, fmt.Errorf("parse voices endpoint: %w", err)
	}
	query := endpoint.Query()
	if code := strings.TrimSpace(languageCode); code != "" {
		query.Set("languageCode", code)
	}
	query.Set("key", g.apiKey)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create voices request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request voices: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read voices response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google cloud voices returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload googleCloudVoicesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode voices response: %w", err)
	}

	voices := make([]GoogleCloudVoiceOption, 0, len(payload.Voices))
	for _, voice := range payload.Voices {
		voices = append(voices, GoogleCloudVoiceOption{
			Name:                   voice.Name,
			Tier:                   GoogleCloudVoiceTier(voice.Name),
			LanguageCodes:          voice.LanguageCodes,
			SSMLGender:             voice.SSMLGender,
			NaturalSampleRateHertz: voice.NaturalSampleRateHertz,
		})
	}
	sortGoogleCloudVoices(voices)
	return voices, nil
}

func (g *GoogleCloudTTS) ListRecommendedVoices(ctx context.Context) []GoogleCloudVoiceOption {
	defaults := DefaultGoogleCloudVoices()
	if !g.Enabled() {
		return defaults
	}

	twVoices, err := g.ListVoices(ctx, "cmn-TW")
	if err != nil {
		return defaults
	}

	selected := filterPreferredGoogleCloudVoices(twVoices)
	if !hasVoiceTier(selected, "Chirp 3: HD") {
		if cnVoices, cnErr := g.ListVoices(ctx, "cmn-CN"); cnErr == nil {
			for _, voice := range cnVoices {
				if voice.Tier == "Chirp 3: HD" {
					selected = append(selected, voice)
				}
			}
		}
	}

	seen := make(map[string]struct{}, len(selected))
	for _, v := range selected {
		seen[strings.ToLower(v.Name)] = struct{}{}
	}
	for _, def := range defaults {
		if !hasVoiceTier(selected, def.Tier) {
			if _, ok := seen[strings.ToLower(def.Name)]; !ok {
				selected = append(selected, def)
				seen[strings.ToLower(def.Name)] = struct{}{}
			}
		}
	}

	if len(selected) == 0 {
		return defaults
	}

	sortGoogleCloudVoices(selected)
	return selected
}

func DefaultGoogleCloudVoices() []GoogleCloudVoiceOption {
	return []GoogleCloudVoiceOption{
		{
			Name:          "cmn-TW-Wavenet-A",
			Tier:          "WaveNet",
			LanguageCodes: []string{"cmn-TW"},
			SSMLGender:    "FEMALE",
		},
		{
			Name:          "cmn-TW-Wavenet-B",
			Tier:          "WaveNet",
			LanguageCodes: []string{"cmn-TW"},
			SSMLGender:    "MALE",
		},
		{
			Name:          "cmn-TW-Wavenet-C",
			Tier:          "WaveNet",
			LanguageCodes: []string{"cmn-TW"},
			SSMLGender:    "MALE",
		},
		{
			Name:          "cmn-TW-Neural2-A",
			Tier:          "Neural2",
			LanguageCodes: []string{"cmn-TW"},
			SSMLGender:    "FEMALE",
		},
		{
			Name:          "cmn-CN-Chirp3-HD-Aoede",
			Tier:          "Chirp 3: HD",
			LanguageCodes: []string{"cmn-CN"},
			SSMLGender:    "FEMALE",
		},
	}
}

func GoogleCloudVoiceTier(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "chirp3-hd"):
		return "Chirp 3: HD"
	case strings.Contains(lower, "neural2"):
		return "Neural2"
	case strings.Contains(lower, "wavenet"):
		return "WaveNet"
	case strings.Contains(lower, "standard"):
		return "Standard"
	default:
		return ""
	}
}

func filterPreferredGoogleCloudVoices(voices []GoogleCloudVoiceOption) []GoogleCloudVoiceOption {
	filtered := make([]GoogleCloudVoiceOption, 0, len(voices))
	for _, voice := range voices {
		switch voice.Tier {
		case "Chirp 3: HD", "Neural2", "WaveNet":
			filtered = append(filtered, voice)
		}
	}
	return filtered
}

func hasVoiceTier(voices []GoogleCloudVoiceOption, tier string) bool {
	for _, voice := range voices {
		if voice.Tier == tier {
			return true
		}
	}
	return false
}

func (g *GoogleCloudTTS) SynthesizeToMediaSource(ctx context.Context, req BroadcastRequest) (string, error) {
	if !g.Enabled() {
		return "", fmt.Errorf("google cloud tts is not configured")
	}

	requestBody, err := g.buildSynthesizeRequest(req)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(g.baseURL, "/") + "/text:synthesize?key=" + url.QueryEscape(g.apiKey)
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal google cloud tts payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create google cloud tts request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request google cloud tts: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read google cloud tts response: %w", err)
	}
	if resp.StatusCode == http.StatusBadRequest && strings.EqualFold(strings.TrimSpace(req.Voice), "cmn-TW-Neural2-A") {
		fallbackReq := req
		fallbackReq.Voice = "cmn-TW-Wavenet-A"
		fallbackReq.Language = "cmn-TW"
		return g.SynthesizeToMediaSource(ctx, fallbackReq)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("google cloud tts returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var synthResp googleCloudSynthesizeResponse
	if err := json.Unmarshal(body, &synthResp); err != nil {
		return "", fmt.Errorf("decode google cloud tts response: %w", err)
	}
	if strings.TrimSpace(synthResp.AudioContent) == "" {
		return "", fmt.Errorf("google cloud tts returned empty audioContent")
	}

	audio, err := base64.StdEncoding.DecodeString(synthResp.AudioContent)
	if err != nil {
		return "", fmt.Errorf("decode google cloud audioContent: %w", err)
	}

	if err := os.MkdirAll(g.mediaDir, 0o755); err != nil {
		return "", fmt.Errorf("create google cloud media directory: %w", err)
	}
	if err := pruneGeneratedAudio(g.mediaDir, 24*time.Hour); err != nil {
		return "", err
	}

	filename := "tts-" + time.Now().Format("20060102-150405") + "-" + randomToken(6) + ".mp3"
	path := filepath.Join(g.mediaDir, filename)
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return "", fmt.Errorf("write google cloud audio file: %w", err)
	}

	return g.mediaSourcePrefix + filename, nil
}

func (g *GoogleCloudTTS) buildSynthesizeRequest(req BroadcastRequest) (*googleCloudSynthesizeRequest, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	voiceName := strings.TrimSpace(req.Voice)
	isChirp3HD := GoogleCloudVoiceTier(voiceName) == "Chirp 3: HD"

	inputMode := strings.ToLower(strings.TrimSpace(req.InputMode))
	if inputMode == "" || isChirp3HD {
		inputMode = "text"
	}

	input := googleCloudSynthesizeInput{}
	switch inputMode {
	case "text":
		input.Text = message
	case "ssml":
		input.SSML = message
	default:
		return nil, fmt.Errorf("unsupported input_mode: %s", req.InputMode)
	}

	languageCode := deriveLanguageCodeFromVoice(voiceName)
	if languageCode == "" {
		languageCode = strings.TrimSpace(req.Language)
	}
	if languageCode == "" {
		languageCode = "cmn-TW"
	}

	voice := googleCloudSynthesizeVoice{
		LanguageCode: languageCode,
	}
	if voiceName != "" {
		voice.Name = voiceName
	}

	audioConfig := googleCloudSynthesizeAudioConfig{
		AudioEncoding: "MP3",
	}
	if !isChirp3HD {
		if req.SpeakingRate > 0 {
			audioConfig.SpeakingRate = req.SpeakingRate
		}
		if req.Pitch != 0 {
			audioConfig.Pitch = req.Pitch
		}
	}
	if req.VolumeGainDB != 0 {
		audioConfig.VolumeGainDb = req.VolumeGainDB
	}
	if profiles := normalizeEffectsProfiles(req.EffectsProfileID); len(profiles) > 0 {
		audioConfig.EffectsProfileID = profiles
	}

	return &googleCloudSynthesizeRequest{
		Input:       input,
		Voice:       voice,
		AudioConfig: audioConfig,
	}, nil
}

func normalizeEffectsProfiles(raw string) []string {
	parts := strings.Split(raw, ",")
	profiles := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		profiles = append(profiles, value)
	}
	return profiles
}

func pruneGeneratedAudio(dir string, maxAge time.Duration) error {
	if maxAge <= 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read generated audio directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
	return nil
}

func randomToken(byteLen int) string {
	if byteLen <= 0 {
		byteLen = 6
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func deriveLanguageCodeFromVoice(voiceName string) string {
	parts := strings.Split(strings.TrimSpace(voiceName), "-")
	if len(parts) < 3 {
		return ""
	}
	return parts[0] + "-" + parts[1]
}

func sortGoogleCloudVoices(voices []GoogleCloudVoiceOption) {
	if len(voices) < 2 {
		return
	}
	tierRank := func(tier string) int {
		switch tier {
		case "WaveNet":
			return 0
		case "Neural2":
			return 1
		case "Chirp 3: HD":
			return 2
		default:
			return 3
		}
	}
	localeRank := func(name string) int {
		if strings.HasPrefix(strings.ToLower(name), "cmn-tw-") {
			return 0
		}
		return 1
	}
	sort.Slice(voices, func(i, j int) bool {
		if localeRank(voices[i].Name) != localeRank(voices[j].Name) {
			return localeRank(voices[i].Name) < localeRank(voices[j].Name)
		}
		if tierRank(voices[i].Tier) != tierRank(voices[j].Tier) {
			return tierRank(voices[i].Tier) < tierRank(voices[j].Tier)
		}
		return strings.ToLower(voices[i].Name) < strings.ToLower(voices[j].Name)
	})
}
