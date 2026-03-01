package http

import "net/http"

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (e *AppError) Error() string {
	return e.Message
}

var (
	ErrNotFound = &AppError{
		Code:    "NOT_FOUND",
		Message: "Resource not found",
		Status:  http.StatusNotFound,
	}
	ErrAuthRequired = &AppError{
		Code:    "AUTH_REQUIRED",
		Message: "Authentication required",
		Status:  http.StatusUnauthorized,
	}
	ErrForbidden = &AppError{
		Code:    "FORBIDDEN",
		Message: "Access denied",
		Status:  http.StatusForbidden,
	}
	ErrValidation = &AppError{
		Code:    "VALIDATION_ERROR",
		Message: "Validation failed",
		Status:  http.StatusUnprocessableEntity,
	}
	ErrConflict = &AppError{
		Code:    "CONFLICT",
		Message: "Resource conflict",
		Status:  http.StatusConflict,
	}
)

func NewAppError(code string, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, Status: status}
}

func NewValidationError(message string) *AppError {
	return &AppError{Code: "VALIDATION_ERROR", Message: message, Status: http.StatusUnprocessableEntity}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: message, Status: http.StatusNotFound}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: message, Status: http.StatusForbidden}
}
