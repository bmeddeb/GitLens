package handlers

import (
	"net/http"

	"github.com/user/gitlens-pro/internal/database"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	apphttp.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}

	if err := database.Ping(r.Context(), h.db); err != nil {
		checks["database"] = "error: " + err.Error()
		apphttp.JSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "not ready",
			"checks": checks,
		})
		return
	}
	checks["database"] = "ok"

	apphttp.JSON(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
		"checks": checks,
	})
}
