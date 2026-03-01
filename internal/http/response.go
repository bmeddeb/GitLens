package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Cursor     string `json:"cursor,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, SuccessResponse{Data: data})
}

func SuccessWithMeta(w http.ResponseWriter, data interface{}, meta *Meta) {
	JSON(w, http.StatusOK, SuccessResponse{Data: data, Meta: meta})
}

func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, SuccessResponse{Data: data})
}

func Accepted(w http.ResponseWriter, data interface{}, location string) {
	w.Header().Set("Location", location)
	JSON(w, http.StatusAccepted, SuccessResponse{Data: data})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func RespondError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*AppError); ok {
		JSON(w, appErr.Status, ErrorResponse{
			Error: ErrorBody{
				Code:    appErr.Code,
				Message: appErr.Message,
				Status:  appErr.Status,
			},
		})
		return
	}

	JSON(w, http.StatusInternalServerError, ErrorResponse{
		Error: ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "Internal server error",
			Status:  http.StatusInternalServerError,
		},
	})
}

func EncodeCursor(id uint64) string {
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", id)))
}

func DecodeCursor(cursor string) (uint64, error) {
	if cursor == "" {
		return 0, nil
	}
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}
	return strconv.ParseUint(string(data), 10, 64)
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func ParsePagination(r *http.Request) (cursor uint64, limit int, err error) {
	cursor, err = DecodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		return 0, 0, err
	}

	limit = DefaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, err = strconv.Atoi(l)
		if err != nil || limit < 1 {
			limit = DefaultLimit
		}
		if limit > MaxLimit {
			limit = MaxLimit
		}
	}

	return cursor, limit, nil
}
