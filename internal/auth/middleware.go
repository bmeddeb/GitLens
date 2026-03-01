package auth

import (
	"net/http"

	"github.com/user/gitlens-pro/internal/user"
	"go.uber.org/zap"
)

type SessionMiddleware struct {
	sessionStore SessionStore
	userService  user.Service
	logger       *zap.Logger
}

func NewSessionMiddleware(
	sessionStore SessionStore,
	userService user.Service,
	logger *zap.Logger,
) *SessionMiddleware {
	return &SessionMiddleware{
		sessionStore: sessionStore,
		userService:  userService,
		logger:       logger,
	}
}

func (m *SessionMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":{"code":"AUTH_REQUIRED","message":"Authentication required","status":401}}`, http.StatusUnauthorized)
			return
		}

		session, err := m.sessionStore.Get(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, `{"error":{"code":"AUTH_REQUIRED","message":"Invalid or expired session","status":401}}`, http.StatusUnauthorized)
			return
		}

		u, err := m.userService.GetByID(r.Context(), session.UserID)
		if err != nil {
			m.logger.Error("session references missing user",
				zap.Uint64("user_id", session.UserID),
				zap.Error(err),
			)
			http.Error(w, `{"error":{"code":"AUTH_REQUIRED","message":"User not found","status":401}}`, http.StatusUnauthorized)
			return
		}

		// Touch session asynchronously
		go func() {
			_ = m.sessionStore.Touch(r.Context(), session.ID)
		}()

		ctx := ContextWithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
