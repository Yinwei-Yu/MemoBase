package api

import (
	"net/http"
	"strings"

	"memobase/backend/internal/core"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&authRegistrar{})
}

type authRegistrar struct{}

func (authRegistrar) Register(public *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	public.POST("/auth/login", handleLogin(app))
	authed.GET("/auth/me", handleAuthMe(app))
}

func handleLogin(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid login payload", nil)
			return
		}
		if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "username and password are required", nil)
			return
		}
		user, err := app.VerifyUser(c.Request.Context(), req.Username, req.Password)
		if err != nil {
			util.Unauthorized(c, "invalid credentials")
			return
		}
		token, err := util.SignToken(app.Config.JWTSecret, user.ID, app.Config.TokenTTL)
		if err != nil {
			util.Internal(c, "failed to sign token")
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"access_token":  token,
			"refresh_token": "",
			"expires_in":    int(app.Config.TokenTTL.Seconds()),
			"user": gin.H{
				"user_id":      user.ID,
				"username":     user.Username,
				"display_name": user.DisplayName,
			},
		})
	}
}

func handleAuthMe(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := app.Store.GetUserByID(c.Request.Context(), userIDFrom(c))
		if err != nil {
			util.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"user_id":      user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
		})
	}
}
