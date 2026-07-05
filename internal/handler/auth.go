package handler

import (
	"errors"
	"io"
	"net/http"

	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc service.AuthService
}

func NewAuthHandler(authSvc service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

type RegisterRequest struct {
	Name        string `json:"name"         binding:"required"`
	Email       string `json:"email"        binding:"required,email"`
	Password    string `json:"password"     binding:"required,min=8"`
	PhoneNumber string `json:"phone_number" binding:"required,min=8,max=20"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileRequest struct {
	Name        string `json:"name"         binding:"required"`
	PhoneNumber string `json:"phone_number" binding:"required,min=8,max=20"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new user account with name, email, and password. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest       true  "Registration payload"
// @Success      201   {object}  response.Response     "User created"
// @Failure      400   {object}  response.ErrorResponse
// @Failure      409   {object}  response.ErrorResponse  "Email already registered"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	lang := i18n.FromContext(c)

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), service.RegisterInput{
		Name:        req.Name,
		Email:       req.Email,
		Password:    req.Password,
		PhoneNumber: req.PhoneNumber,
	})
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": i18n.T(lang, "error.email_taken")})
			return
		}
		response.InternalServerError(c, i18n.T(lang, "auth.register.failed"), err)
		return
	}

	response.Created(c, i18n.T(lang, "auth.register.success"), user)
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password, returns a JWT Bearer token. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest          true  "Login credentials"
// @Success      200   {object}  response.Response     "Token and user info"
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse  "Invalid credentials"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	lang := i18n.FromContext(c)

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	token, user, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCreds) {
			response.Unauthorized(c, i18n.T(lang, "error.invalid_creds"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "auth.login.failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "auth.login.success"), gin.H{"token": token, "user": user})
}

// GetProfile godoc
// @Summary      Get profile
// @Description  Returns the authenticated user's profile. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := c.MustGet("user_id").(uint)

	user, err := h.authSvc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, i18n.T(lang, "error.user_not_found"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "auth.profile.get_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "auth.profile.retrieved"), user)
}

// UploadProfilePicture godoc
// @Summary      Upload profile picture
// @Description  Uploads a profile picture (jpg, png, or webp — max 5 MB). Replaces any existing picture. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file  formData  file  true  "Profile picture file"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      503   {object}  response.ErrorResponse  "Storage not configured"
// @Router       /auth/profile/picture [post]
func (h *AuthHandler) UploadProfilePicture(c *gin.Context) {
	const maxSize = 5 << 20 // 5 MB

	lang := i18n.FromContext(c)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, i18n.T(lang, "auth.picture.required"), err)
		return
	}

	if fileHeader.Size > maxSize {
		response.BadRequest(c, i18n.T(lang, "auth.picture.too_large"), nil)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "auth.picture.read_failed"), err)
		return
	}
	defer file.Close()

	// Detect content type from actual bytes, not just the header
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])

	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		response.BadRequest(c, i18n.T(lang, "auth.picture.invalid_type"), nil)
		return
	}

	// Seek back so the full file is uploaded
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		response.InternalServerError(c, i18n.T(lang, "auth.picture.process_failed"), err)
		return
	}

	userID := c.MustGet("user_id").(uint)

	user, err := h.authSvc.UploadProfilePicture(
		c.Request.Context(),
		userID,
		fileHeader.Filename,
		file,
		fileHeader.Size,
		contentType,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrStorageNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": i18n.T(lang, "error.storage_not_configured")})
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, i18n.T(lang, "error.user_not_found"))
		default:
			response.InternalServerError(c, i18n.T(lang, "auth.picture.upload_failed"), err)
		}
		return
	}

	response.OK(c, i18n.T(lang, "auth.picture.uploaded"), user)
}

// UpdateProfile godoc
// @Summary      Update profile
// @Description  Updates the authenticated user's profile name and phone number. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      UpdateProfileRequest  true  "Profile update payload"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Failure      404   {object}  response.ErrorResponse
// @Router       /auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	lang := i18n.FromContext(c)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	userID := c.MustGet("user_id").(uint)

	user, err := h.authSvc.UpdateProfile(c.Request.Context(), userID, service.UpdateProfileInput{
		Name:        req.Name,
		PhoneNumber: req.PhoneNumber,
	})
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, i18n.T(lang, "error.user_not_found"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "auth.profile.update_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "auth.profile.updated"), user)
}

// ChangePassword godoc
// @Summary      Change password
// @Description  Changes the authenticated user's password after verifying the current one. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      ChangePasswordRequest  true  "Current and new password"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse  "Current password is incorrect"
// @Router       /auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	lang := i18n.FromContext(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	userID := c.MustGet("user_id").(uint)

	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrIncorrectPassword):
			response.Unauthorized(c, i18n.T(lang, "error.incorrect_password"))
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, i18n.T(lang, "error.user_not_found"))
		default:
			response.InternalServerError(c, i18n.T(lang, "auth.password.change_failed"), err)
		}
		return
	}

	response.OK(c, i18n.T(lang, "auth.password.changed"), nil)
}

// LogoutAllDevices godoc
// @Summary      Log out of all devices
// @Description  Invalidates every JWT issued before now for this user, including the one used to call this endpoint — the client should discard its token and redirect to login. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.ErrorResponse
// @Router       /auth/logout-all [post]
func (h *AuthHandler) LogoutAllDevices(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := c.MustGet("user_id").(uint)

	if err := h.authSvc.LogoutAllDevices(c.Request.Context(), userID); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, i18n.T(lang, "error.user_not_found"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "auth.logout_all.failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "auth.logout_all.success"), nil)
}
