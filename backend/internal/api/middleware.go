package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"memobase/backend/internal/core"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-Id")
		if reqID == "" {
			reqID = "req_" + uuid.NewString()
		}
		c.Set("request_id", reqID)
		c.Header("X-Request-Id", reqID)
		c.Next()
	}
}

var skipLogPaths = map[string]bool{
	"/api/v1/healthz":         true,
	"/api/v1/readyz":          true,
	"/api/v1/metrics":         true,
	"/api/v1/metrics/summary": true,
}

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		operation := c.FullPath()
		if skipLogPaths[operation] {
			return
		}
		if operation == "" {
			operation = c.Request.URL.Path
		}

		status := c.Writer.Status()
		attrs := []slog.Attr{
			slog.String("request_id", util.RequestID(c)),
			slog.String("operation", operation),
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int64("request_body_bytes", c.Request.ContentLength),
			slog.Int("response_body_bytes", c.Writer.Size()),
		}

		userID, _ := c.Get("user_id")
		if s, ok := userID.(string); ok && s != "" {
			attrs = append(attrs, slog.String("user_id", s))
		}

		for _, e := range c.Errors {
			attrs = append(attrs, slog.String("gin_error", e.Err.Error()))
		}

		switch {
		case status >= 500:
			logger.LogAttrs(nil, slog.LevelError, "http_request", attrs...)
		case status >= 400:
			logger.LogAttrs(nil, slog.LevelWarn, "http_request", attrs...)
		default:
			logger.LogAttrs(nil, slog.LevelDebug, "http_request", attrs...)
		}
	}
}

func AuthRequired(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			app.Logger.Warn("auth_failed",
				slog.String("request_id", util.RequestID(c)),
				slog.String("reason", "missing_bearer_token"),
				slog.String("client_ip", c.ClientIP()),
				slog.String("path", c.Request.URL.Path),
			)
			util.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(app.Config.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			app.Logger.Warn("auth_failed",
				slog.String("request_id", util.RequestID(c)),
				slog.String("reason", "invalid_token"),
				slog.String("client_ip", c.ClientIP()),
				slog.String("path", c.Request.URL.Path),
			)
			util.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			app.Logger.Warn("auth_failed",
				slog.String("request_id", util.RequestID(c)),
				slog.String("reason", "invalid_claims"),
				slog.String("client_ip", c.ClientIP()),
			)
			util.Unauthorized(c, "invalid claims")
			c.Abort()
			return
		}
		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			app.Logger.Warn("auth_failed",
				slog.String("request_id", util.RequestID(c)),
				slog.String("reason", "invalid_subject"),
				slog.String("client_ip", c.ClientIP()),
			)
			util.Unauthorized(c, "invalid subject")
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func Cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id,Idempotency-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
