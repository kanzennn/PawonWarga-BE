package middleware

import (
	"strings"

	"PawonWarga-BE/pkg/jwtutil"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, "authorization header with Bearer token required")
			c.Abort()
			return
		}

		userID, err := jwtutil.ParseToken(strings.TrimPrefix(header, "Bearer "), secret)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
