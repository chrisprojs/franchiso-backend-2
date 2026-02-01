package middleware

import (
	"net/http"
	"strings"
	"time"
	"errors"

	"github.com/chrisprojs/Franchiso/config"
	"github.com/chrisprojs/Franchiso/models"
	"github.com/chrisprojs/Franchiso/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Refactor: accepts next handler and app
func AuthMiddleware(app *config.App, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := utils.ValidateJWT(tokenString, "access")
		if err == nil {
			// Token masih valid
			c.Set("user_id", claims.UserID)
			c.Set("role", claims.Role)
			next(c)
			return
		}

		// If error is due to expiration, check refresh token
		if !errors.Is(err, jwt.ErrTokenExpired) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Get refresh token from cookie
		refreshToken, err := c.Cookie("refresh_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Refresh token not found"})
			return
		}

		refreshClaims, err := utils.ValidateJWT(refreshToken, "refresh")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}

		// Check refresh token in sessions table
		var session models.Session
		err = app.DB.Model(&session).
			Where("refresh_token = ? AND user_id = ?", refreshToken, refreshClaims.UserID).
			Where("expires_at > ?", time.Now()).
			Select()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
			return
		}

		// Generate new access token
		newAccessToken, err := utils.GenerateJWT(refreshClaims.UserID, refreshClaims.Role, "access")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
			return
		}

		// Send new access token to client (can be via header or response body)
		c.Header("X-New-Access-Token", newAccessToken)
		c.Set("user_id", refreshClaims.UserID)
		c.Set("role", refreshClaims.Role)
		next(c)
	}
}
