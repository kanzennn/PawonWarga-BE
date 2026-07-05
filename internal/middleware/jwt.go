package middleware

import (
	"errors"
	"strings"

	"PawonWarga-BE/internal/repository"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/jwtutil"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// JWTAuth validates the Bearer token and also enforces "log out of all
// devices": if the user has a TokenValidAfter timestamp newer than this
// token's IssuedAt, the token is treated as revoked even though it hasn't
// expired yet. That lookup is why this needs a UserRepository, not just the
// JWT secret.
func JWTAuth(secret string, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := i18n.FromContext(c)

		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, i18n.T(lang, "auth.token_required"))
			c.Abort()
			return
		}

		userID, issuedAt, err := jwtutil.ParseToken(strings.TrimPrefix(header, "Bearer "), secret)
		if err != nil {
			response.Unauthorized(c, i18n.T(lang, "auth.token_invalid"))
			c.Abort()
			return
		}

		user, err := userRepo.FindByID(c.Request.Context(), userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Unauthorized(c, i18n.T(lang, "auth.token_invalid"))
			} else {
				response.InternalServerError(c, i18n.T(lang, "auth.token_invalid"), err)
			}
			c.Abort()
			return
		}

		if user.TokenValidAfter != nil && issuedAt.Before(*user.TokenValidAfter) {
			response.Unauthorized(c, i18n.T(lang, "auth.session_revoked"))
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
