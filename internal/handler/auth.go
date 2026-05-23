package handler

import (
	"errors"
	"io"
	"net/http"

	"PawonWarga-BE/internal/service"
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
	Name     string `json:"name"     binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new user account with name, email, and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest       true  "Registration payload"
// @Success      201   {object}  response.Response     "User created"
// @Failure      400   {object}  response.ErrorResponse
// @Failure      409   {object}  response.ErrorResponse  "Email already registered"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), service.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		response.InternalServerError(c, "failed to register", err)
		return
	}

	response.Created(c, "registration successful", user)
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password, returns a JWT Bearer token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest          true  "Login credentials"
// @Success      200   {object}  response.Response     "Token and user info"
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse  "Invalid credentials"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	token, user, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCreds) {
			response.Unauthorized(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to login", err)
		return
	}

	response.OK(c, "login successful", gin.H{"token": token, "user": user})
}

// GetProfile godoc
// @Summary      Get profile
// @Description  Returns the authenticated user's profile
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Router       /auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	user, err := h.authSvc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to get profile", err)
		return
	}

	response.OK(c, "profile retrieved", user)
}

// UploadProfilePicture godoc
// @Summary      Upload profile picture
// @Description  Uploads a profile picture (jpg, png, or webp — max 5 MB). Replaces any existing picture.
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

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required", err)
		return
	}

	if fileHeader.Size > maxSize {
		response.BadRequest(c, "file must be at most 5 MB", nil)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.InternalServerError(c, "failed to read file", err)
		return
	}
	defer file.Close()

	// Detect content type from actual bytes, not just the header
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])

	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		response.BadRequest(c, "only jpg, png, and webp images are allowed", nil)
		return
	}

	// Seek back so the full file is uploaded
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		response.InternalServerError(c, "failed to process file", err)
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
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, service.ErrUserNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalServerError(c, "failed to upload profile picture", err)
		}
		return
	}

	response.OK(c, "profile picture uploaded", user)
}

// UpdateProfile godoc
// @Summary      Update profile
// @Description  Updates the authenticated user's profile name
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
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	userID := c.MustGet("user_id").(uint)

	user, err := h.authSvc.UpdateProfile(c.Request.Context(), userID, service.UpdateProfileInput{
		Name: req.Name,
	})
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalServerError(c, "failed to update profile", err)
		return
	}

	response.OK(c, "profile updated", user)
}
