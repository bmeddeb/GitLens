package handlers

import (
	"net/http"
	"runtime"

	"github.com/user/gitlens-pro/internal/auth"
	"github.com/user/gitlens-pro/internal/database"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

func (h *AdminHandler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if u == nil || u.Role != "admin" {
		apphttp.RespondError(w, apphttp.ErrForbidden)
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	dbOK := "ok"
	if err := database.Ping(r.Context(), h.db); err != nil {
		dbOK = "error: " + err.Error()
	}

	apphttp.Success(w, map[string]interface{}{
		"go_version":   runtime.Version(),
		"goroutines":   runtime.NumGoroutine(),
		"alloc_mb":     memStats.Alloc / 1024 / 1024,
		"sys_mb":       memStats.Sys / 1024 / 1024,
		"num_gc":       memStats.NumGC,
		"database":     dbOK,
	})
}
