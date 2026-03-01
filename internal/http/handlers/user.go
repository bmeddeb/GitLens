package handlers

import (
	"net/http"

	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		apphttp.RespondError(w, apphttp.ErrAuthRequired)
		return
	}

	apphttp.Success(w, map[string]interface{}{
		"id":           u.ID,
		"github_id":    u.GitHubID,
		"username":     u.Username,
		"display_name": u.DisplayName,
		"email":        u.Email,
		"avatar_url":   u.AvatarURL,
		"role":         u.Role,
		"created_at":   u.CreatedAt,
		"last_login_at": u.LastLoginAt,
	})
}
