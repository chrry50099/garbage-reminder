package reminder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testStatusProvider struct {
	status StatusSnapshot
}

func (t testStatusProvider) CurrentStatus() StatusSnapshot {
	return t.status
}

func TestStatusHandlerServesJSON(t *testing.T) {
	handler := NewStatusHandler(testStatusProvider{
		status: StatusSnapshot{
			UpdatedAt:   time.Date(2026, 4, 12, 20, 30, 0, 0, time.UTC),
			Active:      true,
			RouteName:   "雙溪線",
			PointName:   "有謙家園",
			PointSeq:    27,
			ServiceDate: "2026-04-12",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content type, got %s", got)
	}

	var payload StatusSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.RouteName != "雙溪線" || payload.PointSeq != 27 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDashboardHandlerServesHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	NewDashboardHandler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected html content type, got %s", got)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Garbage ETA Predictor") {
		t.Fatalf("expected dashboard title in body, got %s", body)
	}
	if !strings.Contains(body, "statusURL") || !strings.Contains(body, "HomePod Mini 測試播報") || !strings.Contains(body, "歷史資料") {
		t.Fatalf("expected dashboard script in body, got %s", body)
	}
}
