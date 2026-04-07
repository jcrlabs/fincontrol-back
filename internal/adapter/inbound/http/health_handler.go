package http

import (
	"net/http"
	"os"
	"time"
)

var startTime = time.Now()

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	version := "dev"
	if v, err := os.ReadFile("VERSION"); err == nil {
		version = string(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version,
		"uptime":  int64(time.Since(startTime).Seconds()),
	})
}
