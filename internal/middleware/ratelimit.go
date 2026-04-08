package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type slidingWindow struct {
	mu        sync.Mutex
	attempts  []time.Time
	maxHits   int
	windowDur time.Duration
}

func (sw *slidingWindow) allow() bool {
	now := time.Now()
	sw.mu.Lock()
	defer sw.mu.Unlock()

	cutoff := now.Add(-sw.windowDur)
	valid := sw.attempts[:0]
	for _, t := range sw.attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	sw.attempts = valid

	if len(sw.attempts) >= sw.maxHits {
		return false
	}
	sw.attempts = append(sw.attempts, now)
	return true
}

// RateLimiter implements a sliding-window rate limiter keyed by IP.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*slidingWindow
	maxHits int
	window  time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*slidingWindow),
		maxHits: limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) get(key string) *slidingWindow {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	sw, ok := rl.entries[key]
	if !ok {
		sw = &slidingWindow{maxHits: rl.maxHits, windowDur: rl.window}
		rl.entries[key] = sw
	}
	return sw
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for k, sw := range rl.entries {
			sw.mu.Lock()
			if len(sw.attempts) == 0 {
				delete(rl.entries, k)
			}
			sw.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// Limit returns middleware that rate-limits by remote IP using the given limiter.
func Limit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !rl.get(ip).allow() {
				http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
