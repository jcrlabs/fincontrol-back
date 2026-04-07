package middleware

import (
	"net/http"
	"sync"
	"time"
)

type windowEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimiter implements a fixed-window rate limiter keyed by IP.
type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*windowEntry
	limit    int
	duration time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		windows:  make(map[string]*windowEntry),
		limit:    limit,
		duration: window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.windows[key]
	if !ok || now.After(entry.windowEnd) {
		rl.windows[key] = &windowEntry{count: 1, windowEnd: now.Add(rl.duration)}
		return true
	}
	if entry.count >= rl.limit {
		return false
	}
	entry.count++
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for k, v := range rl.windows {
			if now.After(v.windowEnd) {
				delete(rl.windows, k)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns middleware that rate-limits by remote IP using the given limiter.
func Limit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ip = xff
			}
			if !rl.Allow(ip) {
				http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
