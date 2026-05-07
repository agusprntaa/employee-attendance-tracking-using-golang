package utils

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ─── Response Helpers ────────────────────────────────────────────────────────

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   data,
	})
}

func SuccessMessage(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": message,
	})
}

func Created(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "success",
		"data":   data,
	})
}

func ErrorResponse(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]interface{}{
		"status":  "error",
		"code":    code,
		"message": message,
	})
}

func BadRequest(w http.ResponseWriter, code, message string) {
	ErrorResponse(w, http.StatusBadRequest, code, message)
}

func Unauthorized(w http.ResponseWriter, message string) {
	ErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(w http.ResponseWriter, message string) {
	ErrorResponse(w, http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(w http.ResponseWriter, message string) {
	ErrorResponse(w, http.StatusNotFound, "NOT_FOUND", message)
}

func InternalError(w http.ResponseWriter, message string) {
	ErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// ─── Query Helpers ───────────────────────────────────────────────────────────

func ParsePage(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return page, limit
}

func ParseBody(r *http.Request, dst interface{}) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func Offset(page, limit int) int {
	return (page - 1) * limit
}
