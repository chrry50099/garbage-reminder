package reminder

import (
	"encoding/json"
	"net/http"
	"strings"

	"telegram-garbage-reminder/internal/history"
)

type historyProvider interface {
	ListHistoryDates(limit int) ([]string, error)
	LoadTodayHistory() (*history.DayData, error)
	LoadHistoryDay(serviceDate string) (*history.DayData, error)
}

func NewHistoryDatesHandler(provider historyProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dates, err := provider.ListHistoryDates(30)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"dates": dates})
	})
}

func NewHistoryTodayHandler(provider historyProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		day, err := provider.LoadTodayHistory()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(day)
	})
}

func NewHistoryDayHandler(provider historyProvider) http.Handler {
	return newHistoryDayPayloadHandler(provider, "application/json; charset=utf-8", func(day *history.DayData) ([]byte, error) {
		return day.MarshalJSONBytes()
	})
}

func NewHistoryDayJSONHandler(provider historyProvider) http.Handler {
	return newHistoryDayPayloadHandler(provider, "application/json; charset=utf-8", func(day *history.DayData) ([]byte, error) {
		return day.MarshalJSONBytes()
	})
}

func NewHistoryDayCSVHandler(provider historyProvider) http.Handler {
	return newHistoryDayPayloadHandler(provider, "text/csv; charset=utf-8", func(day *history.DayData) ([]byte, error) {
		return day.MarshalCSV()
	})
}

func newHistoryDayPayloadHandler(provider historyProvider, contentType string, encode func(day *history.DayData) ([]byte, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		serviceDate := strings.TrimSpace(r.URL.Query().Get("date"))
		if serviceDate == "" {
			http.Error(w, "date is required", http.StatusBadRequest)
			return
		}

		day, err := provider.LoadHistoryDay(serviceDate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		payload, err := encode(day)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(payload)
	})
}
