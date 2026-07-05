package response

import (
	"errors"
	"math"
	"net/http"
	"strings"

	"PawonWarga-BE/pkg/i18n"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ValidationErrorResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// ValidationFailed translates validator.ValidationErrors into a clean field → message map,
// localized to the request's language (see pkg/i18n).
func ValidationFailed(c *gin.Context, err error) {
	lang := i18n.FromContext(c)

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		BadRequest(c, i18n.T(lang, "validation.invalid_body"), err)
		return
	}

	errs := make(map[string]string, len(ve))
	for _, fe := range ve {
		errs[strings.ToLower(fe.Field())] = validationMessage(lang, fe)
	}

	c.JSON(http.StatusBadRequest, ValidationErrorResponse{
		Success: false,
		Message: i18n.T(lang, "validation.failed"),
		Errors:  errs,
	})
}

func validationMessage(lang i18n.Lang, fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return i18n.T(lang, "validation.required")
	case "email":
		return i18n.T(lang, "validation.email")
	case "min":
		return i18n.T(lang, "validation.min", fe.Param())
	case "max":
		return i18n.T(lang, "validation.max", fe.Param())
	case "len":
		return i18n.T(lang, "validation.len", fe.Param())
	case "numeric":
		return i18n.T(lang, "validation.numeric")
	case "alphanum":
		return i18n.T(lang, "validation.alphanum")
	case "url":
		return i18n.T(lang, "validation.url")
	case "gte":
		return i18n.T(lang, "validation.gte", fe.Param())
	case "lte":
		return i18n.T(lang, "validation.lte", fe.Param())
	case "oneof":
		return i18n.T(lang, "validation.oneof", fe.Param())
	default:
		return i18n.T(lang, "validation.generic", fe.Tag())
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

// OK, Created, BadRequest, Unauthorized, NotFound, InternalServerError, and
// Paginated all take an already-localized message — callers resolve it via
// i18n.T(i18n.FromContext(c), "key") so this package doesn't need to know
// about specific message keys, only how to shape the JSON envelope.

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
