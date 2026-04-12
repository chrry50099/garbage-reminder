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
)

type HomeAssistant struct {
	baseURL    string
	token      string
	mode       string
	target     string
	httpClient *http.Client
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

func (h *HomeAssistant) SendMessage(ctx context.Context, text string) error {
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

	for _, targetEntityID := range request.TargetEntityIDs {
		targetEntityID = strings.TrimSpace(targetEntityID)
		if targetEntityID == "" {
			continue
		}

		payload := map[string]interface{}{
			"entity_id":              ttsEntityID,
			"media_player_entity_id": targetEntityID,
			"message":                message,
			"cache":                  true,
		}
		if language := strings.TrimSpace(request.Language); language != "" {
			payload["language"] = language
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal tts payload: %w", err)
		}

		endpoint := fmt.Sprintf("%s/api/services/tts/speak", h.baseURL)
		resp, err := h.do(ctx, http.MethodPost, endpoint, body)
		if err != nil {
			return fmt.Errorf("send tts speak request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read tts speak response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("tts speak failed for %s with status %d: %s", targetEntityID, resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
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
	preferred := []string{"tts.google_en_com", "tts.google_ai_tts"}
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
