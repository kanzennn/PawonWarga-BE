package middleware

import (
	"crypto/subtle"

	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

// APIKeyAuth validates a shared-secret X-API-Key header using constant-time
// comparison to prevent timing attacks. Used for internal service-to-service
// routes (the Python labeling worker) instead of Basic Auth, so the secret
// can be rotated independently of human/admin credentials.
func APIKeyAuth(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(expectedKey)) != 1 {
			response.Unauthorized(c, "invalid or missing api key")
			c.Abort()
			return
		}

		c.Next()
	}
}
