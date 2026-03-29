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

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Milliseconds()
		operation := c.FullPath()
		if operation == "" {
			operation = c.Request.URL.Path
		}
		logger.Info("http_request",
			slog.String("request_id", util.RequestID(c)),
			slog.String("operation", operation),
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", duration),
		)
	}
}

func AuthRequired(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			util.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(app.Config.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			util.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			util.Unauthorized(c, "invalid claims")
			c.Abort()
			return
		}
		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
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
