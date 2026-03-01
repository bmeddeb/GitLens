package handlers

import (
	"net/http"

	"github.com/user/gitlens-pro/internal/auth"
	authtemplates "github.com/user/gitlens-pro/internal/auth/templates"
	"github.com/user/gitlens-pro/internal/http/templates"
	"github.com/user/gitlens-pro/internal/notification"
)

type PageHandler struct {
	notifService notification.Service
}

func NewPageHandler(notifService notification.Service) *PageHandler {
	return &PageHandler{notifService: notifService}
}

func (h *PageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	var unread int64
	if u != nil {
		unread, _ = h.notifService.CountUnread(r.Context(), u.ID)
	}

	pd := templates.PageData{
		Title:       "Dashboard",
		User:        u,
		CurrentPath: "/",
		UnreadCount: int(unread),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		templates.DashboardContent(pd).Render(r.Context(), w)
	} else {
		templates.DashboardPage(pd).Render(r.Context(), w)
	}
}

func (h *PageHandler) Login(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to dashboard
	if u := auth.UserFromContext(r.Context()); u != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	authtemplates.LoginPage().Render(r.Context(), w)
}

func (h *PageHandler) NotificationBadgeFragment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	var count int64
	if u != nil {
		count, _ = h.notifService.CountUnread(r.Context(), u.ID)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.NotificationBadge(int(count)).Render(r.Context(), w)
}

func (h *PageHandler) Error404(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	templates.Error404().Render(r.Context(), w)
}

func (h *PageHandler) Error403(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	templates.Error403().Render(r.Context(), w)
}

func (h *PageHandler) Error500(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	templates.Error500().Render(r.Context(), w)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
