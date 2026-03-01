package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/user"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	oauth2github "golang.org/x/oauth2/github"
)

const (
	stateCookieName   = "gitlens_oauth_state"
	sessionCookieName = "gitlens_session"
)

type OAuthHandler struct {
	oauthCfg     *oauth2.Config
	sessionStore SessionStore
	userService  user.Service
	encryptor    *Encryptor
	appCfg       *config.AppConfig
	logger       *zap.Logger
}

func NewOAuthHandler(
	cfg *config.AppConfig,
	sessionStore SessionStore,
	userService user.Service,
	encryptor *Encryptor,
	logger *zap.Logger,
) *OAuthHandler {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GitHub.ClientID,
		ClientSecret: cfg.GitHub.ClientSecret,
		Scopes:       cfg.GitHub.Scopes,
		Endpoint:     oauth2github.Endpoint,
		RedirectURL:  cfg.Server.BaseURL + "/auth/github/callback",
	}

	return &OAuthHandler{
		oauthCfg:     oauthCfg,
		sessionStore: sessionStore,
		userService:  userService,
		encryptor:    encryptor,
		appCfg:       cfg,
		logger:       logger,
	}
}

func (h *OAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure(h.appCfg.Server.BaseURL),
	})

	url := h.oauthCfg.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" {
		h.logger.Warn("OAuth callback missing state cookie",
			zap.String("request_host", r.Host),
			zap.String("configured_base_url", h.appCfg.Server.BaseURL),
			zap.String("referer", r.Referer()),
			zap.Error(err),
		)
		http.Error(w, "Missing state cookie — ensure you access the app via the same host as GITLENS_SERVER_BASE_URL", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	token, err := h.oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("OAuth exchange failed", zap.Error(err))
		http.Error(w, "OAuth exchange failed", http.StatusInternalServerError)
		return
	}

	ghClient := github.NewClient(oauth2.NewClient(r.Context(), oauth2.StaticTokenSource(token)))

	ghUser, _, err := ghClient.Users.Get(r.Context(), "")
	if err != nil {
		h.logger.Error("failed to get GitHub user", zap.Error(err))
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	emails, _, err := ghClient.Users.ListEmails(r.Context(), nil)
	if err != nil {
		h.logger.Error("failed to get GitHub emails", zap.Error(err))
		http.Error(w, "Failed to get user emails", http.StatusInternalServerError)
		return
	}

	var primaryEmail string
	for _, e := range emails {
		if e.GetPrimary() && e.GetVerified() {
			primaryEmail = e.GetEmail()
			break
		}
	}

	encryptedToken, err := h.encryptor.Encrypt([]byte(token.AccessToken))
	if err != nil {
		h.logger.Error("failed to encrypt token", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	u, err := h.userService.FindOrCreateByGitHub(r.Context(), &user.GitHubUserInfo{
		GitHubID:    uint64(ghUser.GetID()),
		Username:    ghUser.GetLogin(),
		DisplayName: ghUser.GetName(),
		Email:       primaryEmail,
		AvatarURL:   ghUser.GetAvatarURL(),
		AccessToken: encryptedToken,
	})
	if err != nil {
		h.logger.Error("failed to upsert user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	session, err := h.sessionStore.Create(r.Context(), u.ID, h.appCfg.Access.SessionDuration)
	if err != nil {
		h.logger.Error("failed to create session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure(h.appCfg.Server.BaseURL),
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (h *OAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.sessionStore.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusNoContent)
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func isSecure(baseURL string) bool {
	return len(baseURL) >= 8 && baseURL[:8] == "https://"
}

type contextKey string

const userContextKey contextKey = "user"

func UserFromContext(ctx context.Context) *user.User {
	u, _ := ctx.Value(userContextKey).(*user.User)
	return u
}

func ContextWithUser(ctx context.Context, u *user.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func NewEncryptorFromConfig(cfg *config.AppConfig) (*Encryptor, error) {
	return NewEncryptor(cfg.Encryption.Key)
}

func SessionDuration(cfg *config.AppConfig) time.Duration {
	return cfg.Access.SessionDuration
}
