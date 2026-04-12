package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create home assistant request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
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
