package http

import (
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/user/gitlens-pro/internal/auth"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				logger.Info("http request",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Duration("duration", time.Since(start)),
					zap.Int("bytes", ww.BytesWritten()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

type RateLimiter struct {
	limiters sync.Map
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(requestsPerMin int) *RateLimiter {
	return &RateLimiter{
		rate:  rate.Limit(float64(requestsPerMin) / 60.0),
		burst: requestsPerMin,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitKey(r)

		limiterI, _ := rl.limiters.LoadOrStore(key, rate.NewLimiter(rl.rate, rl.burst))
		limiter := limiterI.(*rate.Limiter)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			JSON(w, http.StatusTooManyRequests, ErrorResponse{
				Error: ErrorBody{
					Code:    "RATE_LIMITED",
					Message: "Too many requests",
					Status:  http.StatusTooManyRequests,
				},
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func rateLimitKey(r *http.Request) string {
	if u := auth.UserFromContext(r.Context()); u != nil {
		return "user:" + string(rune(u.ID))
	}
	return "ip:" + r.RemoteAddr
}

func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("path", r.URL.Path),
					)
					JSON(w, http.StatusInternalServerError, ErrorResponse{
						Error: ErrorBody{
							Code:    "INTERNAL_ERROR",
							Message: "Internal server error",
							Status:  http.StatusInternalServerError,
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
