package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"telegram-garbage-reminder/internal/state"
)

type HomeAssistant struct {
	baseURL    string
	token      string
	mode       string
	target     string
	httpClient *http.Client
	stateStore *state.LocalStore
}

type haMessagePayload struct {
	Message string `json:"message"`
	Source  string `json:"source"`
}

type BroadcastEntityOption struct {
	EntityID     string `json:"entity_id"`
	FriendlyName string `json:"friendly_name"`
	State        string `json:"state"`
}

type BroadcastOptions struct {
	MediaPlayers     []BroadcastEntityOption `json:"media_players"`
	TTSEntities      []BroadcastEntityOption `json:"tts_entities"`
	DefaultTTSEntity string                  `json:"default_tts_entity,omitempty"`
}

type BroadcastRequest struct {
	Message         string   `json:"message"`
	TargetEntityIDs []string `json:"target_entity_ids"`
	TTSEntityID     string   `json:"tts_entity_id,omitempty"`
	Language        string   `json:"language,omitempty"`
	Voice           string   `json:"voice,omitempty"`
}

type AutomaticBroadcastSettings struct {
	TargetEntityIDs []string `json:"target_entity_ids,omitempty"`
	TTSEntityID     string   `json:"tts_entity_id,omitempty"`
	Language        string   `json:"language,omitempty"`
	Voice           string   `json:"voice,omitempty"`
}

type ttsGetURLRequest struct {
	EngineID string                 `json:"engine_id"`
	Message  string                 `json:"message"`
	Cache    bool                   `json:"cache"`
	Language string                 `json:"language,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ttsGetURLResponse struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

type haEntityState struct {
	EntityID   string                 `json:"entity_id"`
	State      string                 `json:"state"`
	Attributes map[string]interface{} `json:"attributes"`
}

func NewHomeAssistant(baseURL, token, mode, target string) *HomeAssistant {
	return &HomeAssistant{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		mode:    strings.TrimSpace(mode),
		target:  strings.TrimSpace(target),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *HomeAssistant) SetStateStore(store *state.LocalStore) {
	h.stateStore = store
}

func (h *HomeAssistant) SendMessage(ctx context.Context, text string) error {
	speechText := summarizeForSpeech(text)
	if err := h.sendDirectSpeech(ctx, speechText); err == nil {
		return nil
	} else {
		if fallbackErr := h.sendLegacyMessage(ctx, speechText); fallbackErr == nil {
			return nil
		} else {
			return fmt.Errorf("direct home assistant playback failed: %v; fallback failed: %w", err, fallbackErr)
		}
	}
}

func (h *HomeAssistant) GetAutoBroadcastSettings(ctx context.Context) (*AutomaticBroadcastSettings, error) {
	options, err := h.ListBroadcastOptions(ctx)
	if err != nil {
		return nil, err
	}

	settings := AutomaticBroadcastSettings{}
	if h.stateStore != nil {
		if saved := h.stateStore.GetAutoBroadcastSettings(); saved != nil {
			settings.TargetEntityIDs = append([]string(nil), saved.TargetEntityIDs...)
			settings.TTSEntityID = strings.TrimSpace(saved.TTSEntityID)
			settings.Language = strings.TrimSpace(saved.Language)
			settings.Voice = strings.TrimSpace(saved.Voice)
		}
	}

	if settings.TTSEntityID == "" {
		settings.TTSEntityID = options.DefaultTTSEntity
	}
	if strings.EqualFold(settings.TTSEntityID, "tts.google_ai_tts") || strings.EqualFold(settings.TTSEntityID, "tts.google_generative_ai_tts") {
		if settings.Voice == "" {
			settings.Voice = "achernar"
		}
		settings.Language = ""
	}
	if len(settings.TargetEntityIDs) == 0 {
		settings.TargetEntityIDs = h.defaultAutomaticTargetEntityIDs(options)
	}

	return &settings, nil
}

func (h *HomeAssistant) SaveAutoBroadcastSettings(ctx context.Context, settings AutomaticBroadcastSettings) (*AutomaticBroadcastSettings, error) {
	options, err := h.ListBroadcastOptions(ctx)
	if err != nil {
		return nil, err
	}

	normalized := AutomaticBroadcastSettings{
		TargetEntityIDs: normalizeEntityIDs(settings.TargetEntityIDs, "media_player."),
		TTSEntityID:     strings.TrimSpace(settings.TTSEntityID),
		Language:        normalizeLanguageCode(settings.Language),
		Voice:           strings.TrimSpace(settings.Voice),
	}
	if normalized.TTSEntityID == "" {
		normalized.TTSEntityID = options.DefaultTTSEntity
	}
	if !containsEntity(options.TTSEntities, normalized.TTSEntityID) {
		return nil, fmt.Errorf("unsupported tts entity: %s", normalized.TTSEntityID)
	}
	if len(normalized.TargetEntityIDs) == 0 {
		normalized.TargetEntityIDs = h.defaultAutomaticTargetEntityIDs(options)
	}
	if len(normalized.TargetEntityIDs) == 0 {
		return nil, fmt.Errorf("at least one media_player target is required")
	}
	if !containsAllEntities(options.MediaPlayers, normalized.TargetEntityIDs) {
		return nil, fmt.Errorf("one or more media_player targets are unavailable")
	}
	if !supportsExplicitLanguage(normalized.TTSEntityID) {
		normalized.Language = ""
	}
	if resolveVoiceOption(normalized.TTSEntityID, normalized.Voice) == "" {
		normalized.Voice = ""
	} else {
		normalized.Voice = resolveVoiceOption(normalized.TTSEntityID, normalized.Voice)
	}

	if h.stateStore != nil {
		if err := h.stateStore.SaveAutoBroadcastSettings(state.AutoBroadcastSettings{
			TargetEntityIDs: normalized.TargetEntityIDs,
			TTSEntityID:     normalized.TTSEntityID,
			Language:        normalized.Language,
			Voice:           normalized.Voice,
		}); err != nil {
			return nil, fmt.Errorf("persist automatic broadcast settings: %w", err)
		}
	}

	return &normalized, nil
}

func summarizeForSpeech(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if !strings.Contains(text, "垃圾車提醒") {
		return text
	}

	var (
		offsetText    string
		pointName     string
		remainingText string
	)

	lines := strings.Split(text, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.Contains(line, "垃圾車提醒（") && strings.Contains(line, "分鐘門檻"):
			start := strings.Index(line, "（")
			end := strings.Index(line, "分鐘門檻")
			if start >= 0 && end > start {
				offsetText = strings.TrimSpace(line[start+len("（") : end])
			}
		case strings.HasPrefix(line, "站點："):
			value := strings.TrimSpace(strings.TrimPrefix(line, "站點："))
			if idx := strings.Index(value, "（"); idx >= 0 {
				value = strings.TrimSpace(value[:idx])
			}
			pointName = value
		case strings.HasPrefix(line, "剩餘時間："):
			value := strings.TrimSpace(strings.TrimPrefix(line, "剩餘時間："))
			value = strings.TrimSuffix(value, "分鐘")
			value = strings.TrimSpace(value)
			if value != "" {
				remainingText = value
			}
		}
	}

	if remainingText == "" {
		remainingText = offsetText
	}
	if remainingText == "" {
		remainingText = "幾"
	}
	if pointName == "" {
		pointName = "指定站點"
	}

	return fmt.Sprintf("垃圾車快到了，約 %s 分鐘後到 %s，請準備倒垃圾。", remainingText, pointName)
}

func (h *HomeAssistant) ListBroadcastOptions(ctx context.Context) (*BroadcastOptions, error) {
	endpoint := fmt.Sprintf("%s/api/states", h.baseURL)
	resp, err := h.do(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("list home assistant states: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read home assistant states: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("home assistant returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var states []haEntityState
	if err := json.Unmarshal(respBody, &states); err != nil {
		return nil, fmt.Errorf("decode home assistant states: %w", err)
	}

	ttsEntities := make([]BroadcastEntityOption, 0)
	allPlayers := make([]BroadcastEntityOption, 0)
	homePodCandidates := make([]BroadcastEntityOption, 0)
	for _, state := range states {
		option := BroadcastEntityOption{
			EntityID:     state.EntityID,
			FriendlyName: friendlyName(state),
			State:        strings.TrimSpace(state.State),
		}

		switch {
		case strings.HasPrefix(state.EntityID, "tts."):
			ttsEntities = append(ttsEntities, option)
		case strings.HasPrefix(state.EntityID, "media_player."):
			allPlayers = append(allPlayers, option)
			if looksLikeHomePod(option) {
				homePodCandidates = append(homePodCandidates, option)
			}
		}
	}

	sortOptions(ttsEntities)
	sortOptions(allPlayers)
	sortOptions(homePodCandidates)

	players := homePodCandidates
	if len(players) == 0 {
		players = allPlayers
	}

	return &BroadcastOptions{
		MediaPlayers:     players,
		TTSEntities:      ttsEntities,
		DefaultTTSEntity: preferredTTSEntity(ttsEntities),
	}, nil
}

func (h *HomeAssistant) SendTestBroadcast(ctx context.Context, request BroadcastRequest) error {
	message := strings.TrimSpace(request.Message)
	if message == "" {
		return fmt.Errorf("message is required")
	}
	if len(request.TargetEntityIDs) == 0 {
		return fmt.Errorf("at least one target_entity_id is required")
	}

	ttsEntityID := strings.TrimSpace(request.TTSEntityID)
	if ttsEntityID == "" {
		options, err := h.ListBroadcastOptions(ctx)
		if err != nil {
			return fmt.Errorf("resolve default tts entity: %w", err)
		}
		ttsEntityID = options.DefaultTTSEntity
		if ttsEntityID == "" && len(options.TTSEntities) > 0 {
			ttsEntityID = options.TTSEntities[0].EntityID
		}
	}
	if ttsEntityID == "" {
		return fmt.Errorf("no TTS entity available")
	}

	mediaURL, usedTTSEntityID, err := h.resolveBroadcastMediaURL(ctx, request, ttsEntityID)
	if err != nil {
		return err
	}

	for _, targetEntityID := range request.TargetEntityIDs {
		targetEntityID = strings.TrimSpace(targetEntityID)
		if targetEntityID == "" {
			continue
		}

		payload := map[string]interface{}{
			"entity_id":          targetEntityID,
			"media_content_id":   mediaURL,
			"media_content_type": "music",
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal media player payload: %w", err)
		}

		endpoint := fmt.Sprintf("%s/api/services/media_player/play_media", h.baseURL)
		resp, err := h.do(ctx, http.MethodPost, endpoint, body)
		if err != nil {
			return fmt.Errorf("send play_media request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read play_media response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("play_media failed for %s using %s with status %d: %s", targetEntityID, usedTTSEntityID, resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
	}

	return nil
}

func (h *HomeAssistant) sendDirectSpeech(ctx context.Context, text string) error {
	settings, err := h.GetAutoBroadcastSettings(ctx)
	if err != nil {
		return err
	}
	if len(settings.TargetEntityIDs) == 0 {
		return fmt.Errorf("no media_player targets available for automatic playback")
	}

	return h.SendTestBroadcast(ctx, BroadcastRequest{
		Message:         text,
		TargetEntityIDs: settings.TargetEntityIDs,
		TTSEntityID:     settings.TTSEntityID,
		Language:        settings.Language,
		Voice:           settings.Voice,
	})
}

func (h *HomeAssistant) resolveBroadcastMediaURL(ctx context.Context, request BroadcastRequest, primaryTTSEntityID string) (string, string, error) {
	candidates := []string{primaryTTSEntityID}
	options, err := h.ListBroadcastOptions(ctx)
	if err == nil {
		for _, option := range options.TTSEntities {
			if option.EntityID == "" || option.EntityID == primaryTTSEntityID {
				continue
			}
			candidates = append(candidates, option.EntityID)
		}
	}

	var failures []string
	for _, ttsEntityID := range candidates {
		mediaURL, err := h.generateTTSMediaURL(ctx, ttsEntityID, request)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s generate: %v", ttsEntityID, err))
			continue
		}
		if err := h.validateMediaURL(ctx, mediaURL); err != nil {
			failures = append(failures, fmt.Sprintf("%s proxy: %v", ttsEntityID, err))
			continue
		}
		return mediaURL, ttsEntityID, nil
	}

	if len(failures) == 0 {
		return "", "", fmt.Errorf("no usable TTS engine found")
	}
	return "", "", fmt.Errorf("no usable TTS engine found: %s", strings.Join(failures, "; "))
}

func (h *HomeAssistant) sendLegacyMessage(ctx context.Context, text string) error {
	body, err := json.Marshal(haMessagePayload{
		Message: text,
		Source:  "garbage-tracing",
	})
	if err != nil {
		return fmt.Errorf("marshal home assistant payload: %w", err)
	}

	endpoint, err := h.endpoint()
	if err != nil {
		return err
	}

	resp, err := h.do(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return fmt.Errorf("send home assistant request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read home assistant response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("home assistant returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func (h *HomeAssistant) generateTTSMediaURL(ctx context.Context, ttsEntityID string, request BroadcastRequest) (string, error) {
	payload := ttsGetURLRequest{
		EngineID: ttsEntityID,
		Message:  strings.TrimSpace(request.Message),
		Cache:    true,
	}
	if language := normalizeLanguageCode(request.Language); language != "" && supportsExplicitLanguage(ttsEntityID) {
		payload.Language = language
	}
	if voice := resolveVoiceOption(ttsEntityID, request.Voice); voice != "" {
		payload.Options = map[string]interface{}{
			"voice": voice,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal tts_get_url payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/tts_get_url", h.baseURL)
	resp, err := h.do(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("request tts_get_url: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read tts_get_url response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tts_get_url returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var payloadResp ttsGetURLResponse
	if err := json.Unmarshal(respBody, &payloadResp); err != nil {
		return "", fmt.Errorf("decode tts_get_url response: %w", err)
	}
	if strings.TrimSpace(payloadResp.URL) == "" {
		return "", fmt.Errorf("tts_get_url returned empty url")
	}
	return strings.TrimSpace(payloadResp.URL), nil
}

func (h *HomeAssistant) validateMediaURL(ctx context.Context, mediaURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return fmt.Errorf("create media validation request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch generated audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("generated audio returned status %d", resp.StatusCode)
	}

	return nil
}

func (h *HomeAssistant) endpoint() (string, error) {
	switch h.mode {
	case "webhook":
		return fmt.Sprintf("%s/api/webhook/%s", h.baseURL, h.target), nil
	case "service_call":
		parts := strings.SplitN(h.target, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("HA_TTS_TARGET must use domain.service when HA_NOTIFY_MODE=service_call")
		}
		return fmt.Sprintf("%s/api/services/%s/%s", h.baseURL, parts[0], parts[1]), nil
	default:
		return "", fmt.Errorf("unsupported home assistant notify mode: %s", h.mode)
	}
}

func (h *HomeAssistant) do(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create home assistant request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func friendlyName(state haEntityState) string {
	if raw, ok := state.Attributes["friendly_name"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return state.EntityID
}

func looksLikeHomePod(option BroadcastEntityOption) bool {
	value := strings.ToLower(option.EntityID + " " + option.FriendlyName)
	for _, keyword := range []string{"homepod", "hompod", "pod", "mini"} {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func preferredTTSEntity(options []BroadcastEntityOption) string {
	preferred := []string{"tts.google_ai_tts", "tts.google_generative_ai_tts", "tts.google_en_com"}
	for _, entityID := range preferred {
		for _, option := range options {
			if option.EntityID == entityID {
				return option.EntityID
			}
		}
	}
	if len(options) == 0 {
		return ""
	}
	return options[0].EntityID
}

func sortOptions(options []BroadcastEntityOption) {
	sort.Slice(options, func(i, j int) bool {
		left := strings.ToLower(options[i].FriendlyName + options[i].EntityID)
		right := strings.ToLower(options[j].FriendlyName + options[j].EntityID)
		return left < right
	})
}

func supportsExplicitLanguage(entityID string) bool {
	value := strings.ToLower(strings.TrimSpace(entityID))
	return value != "tts.google_ai_tts" && value != "tts.google_generative_ai_tts"
}

func resolveVoiceOption(entityID, requestedVoice string) string {
	value := strings.TrimSpace(requestedVoice)
	switch strings.ToLower(strings.TrimSpace(entityID)) {
	case "tts.google_ai_tts", "tts.google_generative_ai_tts":
		if value != "" {
			return value
		}
		return "achernar"
	default:
		return ""
	}
}

func normalizeLanguageCode(value string) string {
	language := strings.TrimSpace(value)
	if language == "" {
		return ""
	}

	language = strings.ReplaceAll(language, "_", "-")
	language = strings.ToLower(language)
	return language
}

func (h *HomeAssistant) defaultAutomaticTargetEntityIDs(options *BroadcastOptions) []string {
	if configured := normalizeEntityIDs(strings.Split(h.target, ","), "media_player."); len(configured) > 0 {
		return configured
	}

	targets := make([]string, 0, len(options.MediaPlayers))
	for _, option := range options.MediaPlayers {
		entityID := strings.TrimSpace(option.EntityID)
		if entityID == "" {
			continue
		}
		targets = append(targets, entityID)
	}
	return targets
}

func normalizeEntityIDs(values []string, prefix string) []string {
	targets := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		entityID := strings.TrimSpace(value)
		if !strings.HasPrefix(entityID, prefix) {
			continue
		}
		if _, ok := seen[entityID]; ok {
			continue
		}
		seen[entityID] = struct{}{}
		targets = append(targets, entityID)
	}
	return targets
}

func containsEntity(options []BroadcastEntityOption, entityID string) bool {
	for _, option := range options {
		if option.EntityID == entityID {
			return true
		}
	}
	return false
}

func containsAllEntities(options []BroadcastEntityOption, entityIDs []string) bool {
	for _, entityID := range entityIDs {
		if !containsEntity(options, entityID) {
			return false
		}
	}
	return true
}
