package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func RequestID(c *gin.Context) string {
	id := c.GetString("request_id")
	if id == "" {
		id = "req_unknown"
	}
	return id
}

func Success(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{
		"data":       data,
		"request_id": RequestID(c),
		"timestamp":  nowUTC(),
	})
}

func Fail(c *gin.Context, status int, code, message string, details interface{}) {
	c.JSON(status, gin.H{
		"error": APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		"request_id": RequestID(c),
		"timestamp":  nowUTC(),
	})
}

func BadRequest(c *gin.Context, code, message string, details interface{}) {
	Fail(c, http.StatusBadRequest, code, message, details)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Internal(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, "INTERNAL", message, nil)
}
