package middleware

import (
	"crypto/subtle"

	"PawonWarga-BE/internal/config"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

// BasicAuth validates HTTP Basic Auth credentials using constant-time comparison
// to prevent timing attacks.
func BasicAuth(cfg *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="PawonWarga API"`)
			response.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}

		usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
		passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1

		if !usernameOK || !passwordOK {
			c.Header("WWW-Authenticate", `Basic realm="PawonWarga API"`)
			response.Unauthorized(c, "invalid credentials")
			c.Abort()
			return
		}

		c.Set("auth_user", username)
		c.Next()
	}
}
