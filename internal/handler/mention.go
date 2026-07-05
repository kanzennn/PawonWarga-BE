package handler

import (
	"errors"
	"strconv"

	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type MentionHandler struct {
	mentionSvc service.MentionService
}

func NewMentionHandler(mentionSvc service.MentionService) *MentionHandler {
	return &MentionHandler{mentionSvc: mentionSvc}
}

type ListPostsQuery struct {
	Platform  string `form:"platform"`
	Sentiment string `form:"sentiment"`
	Category  string `form:"category"`
	Query     string `form:"q"`
	Page      int    `form:"page,default=1"`
	PerPage   int    `form:"per_page,default=20"`
}

// List godoc
// @Summary      List posts
// @Description  Lists crawled, sentiment-labeled posts with optional filters. Sentiment is null until the nightly labeling worker processes a post. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         mentions
// @Produce      json
// @Security     BearerAuth
// @Param        platform   query     string  false  "Filter by platform (x, instagram, tiktok, youtube, news)"
// @Param        sentiment  query     string  false  "Filter by sentiment (positive, negative, neutral)"
// @Param        category   query     string  false  "Filter by category"
// @Param        q          query     string  false  "Free-text search in content/author"
// @Param        page       query     int     false  "Page number"      default(1)
// @Param        per_page   query     int     false  "Items per page"   default(20)
// @Success      200  {object}  response.PaginatedResponse
// @Failure      400  {object}  response.ErrorResponse
// @Router       /mentions/posts [get]
func (h *MentionHandler) List(c *gin.Context) {
	lang := i18n.FromContext(c)

	var query ListPostsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	posts, total, err := h.mentionSvc.ListPosts(c.Request.Context(), service.ListPostsInput{
		Platform:  query.Platform,
		Sentiment: query.Sentiment,
		Category:  query.Category,
		Query:     query.Query,
		Page:      query.Page,
		PerPage:   query.PerPage,
	})
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "mentions.posts.list_failed"), err)
		return
	}

	response.Paginated(c, i18n.T(lang, "mentions.posts.list_ok"), posts, response.NewPagination(query.Page, query.PerPage, total))
}

// GetByID godoc
// @Summary      Get post detail
// @Description  Returns a single post with its comments. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         mentions
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Post ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.ErrorResponse
// @Router       /mentions/posts/{id} [get]
func (h *MentionHandler) GetByID(c *gin.Context) {
	lang := i18n.FromContext(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, i18n.T(lang, "mentions.posts.invalid_id"), err)
		return
	}

	post, err := h.mentionSvc.GetPost(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, i18n.T(lang, "error.post_not_found"))
			return
		}
		response.InternalServerError(c, i18n.T(lang, "mentions.posts.get_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "mentions.posts.get_ok"), post)
}
