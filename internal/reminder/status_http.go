package reminder

import (
	"encoding/json"
	"net/http"
)

type statusProvider interface {
	CurrentStatus() StatusSnapshot
}

func NewStatusHandler(provider statusProvider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(provider.CurrentStatus())
	})
}
