package auth

import (
	"fmt"
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

// authenticate validates the session cookie and returns the authenticated user.
func (m *SessionMiddleware) authenticate(r *http.Request) (*user.User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("no session cookie")
	}

	session, err := m.sessionStore.Get(r.Context(), cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	u, err := m.userService.GetByID(r.Context(), session.UserID)
	if err != nil {
		m.logger.Error("session references missing user",
			zap.Uint64("user_id", session.UserID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("user not found")
	}

	// Touch session asynchronously
	go func() {
		_ = m.sessionStore.Touch(r.Context(), session.ID)
	}()

	return u, nil
}

// RequireAuth returns 401 JSON for unauthenticated API requests.
func (m *SessionMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := m.authenticate(r)
		if err != nil {
			http.Error(w, `{"error":{"code":"AUTH_REQUIRED","message":"Authentication required","status":401}}`, http.StatusUnauthorized)
			return
		}

		ctx := ContextWithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuthPage redirects to /login for unauthenticated page requests.
func (m *SessionMiddleware) RequireAuthPage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := m.authenticate(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		ctx := ContextWithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
