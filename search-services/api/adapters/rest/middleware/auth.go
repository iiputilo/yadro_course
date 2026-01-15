package middleware

import (
	"net/http"
	"strings"
)

type AuthChecker interface {
	ParseToken(string) error
}

func AuthMiddleware(auth AuthChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		const prefix = "Token "
		
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, prefix) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(h[len(prefix):])
			if token == "" || auth.ParseToken(token) != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
