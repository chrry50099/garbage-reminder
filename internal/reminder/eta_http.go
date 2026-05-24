package reminder

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type etaEvaluationProvider interface {
	LoadETAEvaluation(limit int) (*ETAEvaluation, error)
}

func NewETAEvaluationHandler(provider etaEvaluationProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 60
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return
			}
			limit = parsed
		}

		evaluation, err := provider.LoadETAEvaluation(limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(evaluation)
	})
}
