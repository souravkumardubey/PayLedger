package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Limiter interface {
	Allow(key string) bool
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

type TokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   int
	stopCh  chan struct{}
}

func NewTokenBucket(rate float64, burst int) *TokenBucketLimiter {
	tb := &TokenBucketLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		stopCh:  make(chan struct{}),
	}
	go tb.cleanup(10 * time.Minute)
	return tb
}

func (tb *TokenBucketLimiter) Stop() {
	close(tb.stopCh)
}

func (tb *TokenBucketLimiter) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[key]
	if !ok {
		tb.buckets[key] = &bucket{tokens: float64(tb.burst), lastCheck: time.Now()}
		return true
	}

	now := time.Now()
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > float64(tb.burst) {
		b.tokens = float64(tb.burst)
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (tb *TokenBucketLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tb.mu.Lock()
			now := time.Now()
			for ip, b := range tb.buckets {
				if now.Sub(b.lastCheck) > 2*interval {
					delete(tb.buckets, ip)
				}
			}
			tb.mu.Unlock()
		case <-tb.stopCh:
			return
		}
	}
}

func RateLimit(limiter Limiter, next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !limiter.Allow(ip) {
			logger.Warn("rate limit exceeded", "ip", ip, "path", r.URL.Path)
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
