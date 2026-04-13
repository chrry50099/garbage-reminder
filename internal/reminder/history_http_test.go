package reminder

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"telegram-garbage-reminder/internal/history"
)

type testHistoryProvider struct {
	dates []string
	day   *history.DayData
}

func (t testHistoryProvider) ListHistoryDates(limit int) ([]string, error) {
	return t.dates, nil
}

func (t testHistoryProvider) LoadTodayHistory() (*history.DayData, error) {
	return t.day, nil
}

func (t testHistoryProvider) LoadHistoryDay(serviceDate string) (*history.DayData, error) {
	return t.day, nil
}

func TestHistoryDatesHandlerServesDates(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/history/dates", nil)
	recorder := httptest.NewRecorder()

	NewHistoryDatesHandler(testHistoryProvider{dates: []string{"2026-04-13", "2026-04-12"}}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "2026-04-13") {
		t.Fatalf("expected date payload, got %s", body)
	}
}

func TestHistoryDayCSVHandlerServesCSV(t *testing.T) {
	now := time.Date(2026, 4, 13, 19, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/history/day.csv?date=2026-04-13", nil)
	recorder := httptest.NewRecorder()

	NewHistoryDayCSVHandler(testHistoryProvider{
		day: &history.DayData{
			ServiceDate: "2026-04-13",
			Samples: []history.DaySample{
				{CollectedAt: now, GPSAvailable: true, TruckLat: 24.7, TruckLng: 121.0},
			},
		},
	}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/csv") {
		t.Fatalf("expected csv content type, got %s", got)
	}
	if !strings.Contains(recorder.Body.String(), "service_date") {
		t.Fatalf("expected csv header, got %s", recorder.Body.String())
	}
}
