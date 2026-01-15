package middleware

import (
	"context"
	"net/http"
	"time"
)

type RateLimiter struct {
	tokens chan struct{}
}

func NewRateLimiter(rps int) *RateLimiter {
	if rps <= 0 {
		rps = 1
	}
	rl := &RateLimiter{
		tokens: make(chan struct{}, 1),
	}
	rl.tokens <- struct{}{}

	interval := time.Second / time.Duration(rps)
	if interval <= 0 {
		interval = time.Nanosecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return rl
}

func (l *RateLimiter) acquire(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-l.tokens:
		return true
	}
}

func (l *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.acquire(r.Context()) {
			http.Error(w, http.StatusText(http.StatusGatewayTimeout), http.StatusGatewayTimeout)
			return
		}
		next.ServeHTTP(w, r)
	})
}
