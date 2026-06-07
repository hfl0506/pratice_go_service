package util

import (
	"encoding/json"
	"net/http"
	"time"
)

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"data":      data,
		"status":    status < 400,
		"timestamp": time.Now().UTC(),
	})
}
