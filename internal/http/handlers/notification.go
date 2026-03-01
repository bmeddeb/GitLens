package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/notification"
)

type NotificationHandler struct {
	notifService notification.Service
}

func NewNotificationHandler(notifService notification.Service) *NotificationHandler {
	return &NotificationHandler{notifService: notifService}
}

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	notifications, err := h.notifService.List(r.Context(), u.ID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(notifications) > limit {
		nextCursor = apphttp.EncodeCursor(notifications[limit-1].ID)
		notifications = notifications[:limit]
	}

	apphttp.SuccessWithMeta(w, notifications, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid notification ID"))
		return
	}

	if err := h.notifService.MarkRead(r.Context(), id, u.ID); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.NoContent(w)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	if err := h.notifService.MarkAllRead(r.Context(), u.ID); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.NoContent(w)
}
