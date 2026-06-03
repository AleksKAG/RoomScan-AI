package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/httprate"
)

// RealIPMiddleware извлекает реальный IP клиента (учитывая прокси)
func RealIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			ip = strings.Split(r.RemoteAddr, ":")[0]
		}
		r.Header.Set("X-Real-IP", ip)
		next.ServeHTTP(w, r)
	})
}

// RateLimiter ограничивает количество запросов
// Например: 5 запросов на загрузку в минуту на один IP
func RateLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		5, // лимит
		1 * time.Minute, // период
		httprate.WithKeyFuncs(httprate.KeyByRealIP, httprate.KeyByEndpoint),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error": "too many requests, please try again later"}`, http.StatusTooManyRequests)
		}),
	)
}
