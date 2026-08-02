package handler

import (
	"encoding/json"
	"time"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type IngestHandler struct {
	ingestSvc service.IngestService
}

func NewIngestHandler(ingestSvc service.IngestService) *IngestHandler {
	return &IngestHandler{ingestSvc: ingestSvc}
}

type IngestCommentRequest struct {
	PlatformCommentID string  `json:"platform_comment_id" binding:"required"`
	AuthorHandle      *string `json:"author_handle"`
	// Content is not required — a comment can legitimately have no text
	// (sticker/image-only comment).
	Content        string           `json:"content"`
	LikeCount      int              `json:"like_count"`
	PublishedAt    time.Time        `json:"published_at" binding:"required"`
	Sentiment      *model.Sentiment `json:"sentiment" binding:"omitempty,oneof=positive negative neutral"`
	SentimentScore *float32         `json:"sentiment_score"`
	ModelVersion   *string          `json:"model_version"`
	RawPayload     json.RawMessage  `json:"raw_payload"`
}

type IngestPostRequest struct {
	Platform       model.Platform `json:"platform" binding:"required,oneof=x instagram tiktok youtube news"`
	PlatformPostID string         `json:"platform_post_id" binding:"required"`
	AuthorHandle   *string        `json:"author_handle"`
	AuthorName     *string        `json:"author_name"`
	// Content is not required — a post can legitimately have no caption
	// (video/image-only post).
	Content        string                 `json:"content"`
	URL            *string                `json:"url"`
	LikeCount      int                    `json:"like_count"`
	CommentCount   int                    `json:"comment_count"`
	ShareCount     int                    `json:"share_count"`
	ViewCount      int                    `json:"view_count"`
	PublishedAt    time.Time              `json:"published_at" binding:"required"`
	CrawledAt      time.Time              `json:"crawled_at" binding:"required"`
	Sentiment      *model.Sentiment       `json:"sentiment" binding:"omitempty,oneof=positive negative neutral"`
	SentimentScore *float32               `json:"sentiment_score"`
	ModelVersion   *string                `json:"model_version"`
	RawPayload     json.RawMessage        `json:"raw_payload"`
	Comments       []IngestCommentRequest `json:"comments"`
}

// IngestPost godoc
// @Summary      Ingest a crawled (optionally labeled) post with its comments
// @Description  Internal endpoint for the Python sentiment-labeling worker. Upserts by (platform, platform_post_id, published_at); re-ingesting an existing post refreshes engagement counters but never overwrites an already-set sentiment. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         ingest
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body      IngestPostRequest  true  "Post + comments payload"
// @Success      201   {object}  response.Response
// @Failure      400   {object}  response.ErrorResponse
// @Failure      401   {object}  response.ErrorResponse
// @Router       /ingest/posts [post]
func (h *IngestHandler) IngestPost(c *gin.Context) {
	lang := i18n.FromContext(c)

	var req IngestPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	comments := make([]service.IngestCommentInput, len(req.Comments))
	for i, cm := range req.Comments {
		comments[i] = service.IngestCommentInput{
			PlatformCommentID: cm.PlatformCommentID,
			AuthorHandle:      cm.AuthorHandle,
			Content:           cm.Content,
			LikeCount:         cm.LikeCount,
			PublishedAt:       cm.PublishedAt,
			Sentiment:         cm.Sentiment,
			SentimentScore:    cm.SentimentScore,
			ModelVersion:      cm.ModelVersion,
			RawPayload:        cm.RawPayload,
		}
	}

	post, err := h.ingestSvc.IngestPost(c.Request.Context(), service.IngestPostInput{
		Platform:       req.Platform,
		PlatformPostID: req.PlatformPostID,
		AuthorHandle:   req.AuthorHandle,
		AuthorName:     req.AuthorName,
		Content:        req.Content,
		URL:            req.URL,
		LikeCount:      req.LikeCount,
		CommentCount:   req.CommentCount,
		ShareCount:     req.ShareCount,
		ViewCount:      req.ViewCount,
		PublishedAt:    req.PublishedAt,
		CrawledAt:      req.CrawledAt,
		Sentiment:      req.Sentiment,
		SentimentScore: req.SentimentScore,
		ModelVersion:   req.ModelVersion,
		RawPayload:     req.RawPayload,
		Comments:       comments,
	})
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "ingest.posts.create_failed"), err)
		return
	}

	response.Created(c, i18n.T(lang, "ingest.posts.create_ok"), post)
}
