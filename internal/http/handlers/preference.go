package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/notification"
)

type PreferenceHandler struct {
	prefService notification.PreferenceService
}

func NewPreferenceHandler(prefService notification.PreferenceService) *PreferenceHandler {
	return &PreferenceHandler{prefService: prefService}
}

func (h *PreferenceHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	prefs, err := h.prefService.GetPreferences(r.Context(), u.ID)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, prefs)
}

type upsertPrefRequest struct {
	EventType    string `json:"event_type"`
	DeliveryMode string `json:"delivery_mode"`
	SuppressSelf *bool  `json:"suppress_self"`
}

func (h *PreferenceHandler) UpsertPreferences(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	var req upsertPrefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	suppressSelf := true
	if req.SuppressSelf != nil {
		suppressSelf = *req.SuppressSelf
	}

	pref := &notification.NotificationPreference{
		UserID:       u.ID,
		EventType:    req.EventType,
		DeliveryMode: req.DeliveryMode,
		SuppressSelf: suppressSelf,
	}

	if err := h.prefService.UpsertPreference(r.Context(), pref); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, pref)
}
