package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/middleware"
)

// OK sends a successful JSON response.
func OK(c *gin.Context, code int, data interface{}) {
	c.JSON(code, gin.H{"ok": true, "data": data})
}

// ErrorDetail is the structured error payload returned to clients.
type ErrorDetail struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Status    int            `json:"status"`
	Timestamp string         `json:"timestamp"`
	Method    string         `json:"method"`
	Path      string         `json:"path"`
	Details   map[string]any `json:"details,omitempty"`
}

// Fail sends a structured error response.
// It automatically injects request ID, timestamp, method, and path.
// The error code is auto-derived from the HTTP status if not specified via FailWithCode.
func Fail(c *gin.Context, status int, msg string) {
	code := statusToCode(status)
	sendError(c, status, code, msg, nil)
}

// FailWithCode sends a structured error response with an explicit error code.
func FailWithCode(c *gin.Context, status int, code string, msg string) {
	sendError(c, status, code, msg, nil)
}

// FailWithDetails sends a structured error response with extra diagnostic details.
func FailWithDetails(c *gin.Context, status int, code string, msg string, details map[string]any) {
	sendError(c, status, code, msg, details)
}

// FailFromDB inspects a database error and returns a user-friendly response.
// It detects unique constraint violations (23505) and returns 409 Conflict.
func FailFromDB(c *gin.Context, err error, fallbackMsg string) {
	errStr := err.Error()
	if strings.Contains(errStr, "23505") || strings.Contains(errStr, "unique constraint") || strings.Contains(errStr, "duplicate key") {
		// Extract constraint name if possible
		details := map[string]any{"db_error_hint": "duplicate key value violates unique constraint"}
		if idx := strings.Index(errStr, "\""); idx >= 0 {
			if end := strings.Index(errStr[idx+1:], "\""); end >= 0 {
				details["constraint"] = errStr[idx+1 : idx+1+end]
			}
		}
		FailWithDetails(c, 409, ErrCodeDuplicateName, fallbackMsg+": a record with the same unique fields already exists", details)
		return
	}
	// Log full error server-side, return generic message to client
	fmt.Printf("DB ERROR [%s %s]: %v\n", c.Request.Method, c.Request.URL.Path, err)
	FailWithCode(c, 500, ErrCodeDBError, fallbackMsg)
}

func sendError(c *gin.Context, status int, code string, msg string, details map[string]any) {
	rid := middleware.GetRequestID(c)

	e := ErrorDetail{
		Code:      code,
		Message:   msg,
		RequestID: rid,
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Details:   details,
	}

	c.JSON(status, gin.H{"ok": false, "error": e})
}

// statusToCode maps HTTP status to a default error code when none is specified.
func statusToCode(status int) string {
	switch {
	case status == 400:
		return ErrCodeValidation
	case status == 401:
		return ErrCodeAuthRequired
	case status == 403:
		return ErrCodePermDenied
	case status == 404:
		return ErrCodeNotFound
	case status == 409:
		return ErrCodeConflict
	default:
		return ErrCodeInternal
	}
}
