package middleware

import "net/http"

type ConcurrencyLimiter struct {
	sem chan struct{}
}

func NewConcurrencyLimiter(n int) *ConcurrencyLimiter {
	if n <= 0 {
		n = 1
	}
	return &ConcurrencyLimiter{sem: make(chan struct{}, n)}
}

func (l *ConcurrencyLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case l.sem <- struct{}{}:
			defer func() { <-l.sem }()
			next.ServeHTTP(w, r)
		default:
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
	})
}
