package response

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ValidationErrorResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// ValidationFailed translates validator.ValidationErrors into a clean field → message map.
func ValidationFailed(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		BadRequest(c, "invalid request body", err)
		return
	}

	errs := make(map[string]string, len(ve))
	for _, fe := range ve {
		errs[strings.ToLower(fe.Field())] = validationMessage(fe)
	}

	c.JSON(http.StatusBadRequest, ValidationErrorResponse{
		Success: false,
		Message: "validation failed",
		Errors:  errs,
	})
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", fe.Param())
	case "numeric":
		return "must contain only numbers"
	case "alphanum":
		return "must contain only letters and numbers"
	case "url":
		return "must be a valid URL"
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	default:
		return fmt.Sprintf("failed validation on '%s'", fe.Tag())
	}
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type PaginatedResponse struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message"`
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewPagination(page, perPage int, total int64) Pagination {
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	return Pagination{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}
}

func OK(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Response{Success: true, Message: message, Data: data})
}

func Created(c *gin.Context, message string, data any) {
	c.JSON(http.StatusCreated, Response{Success: true, Message: message, Data: data})
}

func BadRequest(c *gin.Context, message string, err error) {
	resp := ErrorResponse{Success: false, Message: message}
	if err != nil {
		resp.Error = err.Error()
	}
	c.JSON(http.StatusBadRequest, resp)
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{Success: false, Message: message})
}

func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Success: false, Message: message})
}

func InternalServerError(c *gin.Context, message string, err error) {
	resp := ErrorResponse{Success: false, Message: message}
	if err != nil {
		resp.Error = err.Error()
	}
	c.JSON(http.StatusInternalServerError, resp)
}

func Paginated(c *gin.Context, message string, data any, pagination Pagination) {
	c.JSON(http.StatusOK, PaginatedResponse{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}
