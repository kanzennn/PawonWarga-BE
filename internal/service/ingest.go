package service

import (
	"context"
	"encoding/json"
	"time"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
)

// IngestCommentInput mirrors model.Comment's crawlable/labelable fields for a
// single comment attached to a post being ingested.
type IngestCommentInput struct {
	PlatformCommentID string
	AuthorHandle       *string
	Content            string
	LikeCount          int
	PublishedAt        time.Time
	Sentiment          *model.Sentiment
	SentimentScore     *float32
	ModelVersion       *string
	RawPayload         json.RawMessage
}

// IngestPostInput mirrors model.Post's crawlable/labelable fields, plus its
// comments. Sentiment fields are optional — nil means "not labeled yet".
type IngestPostInput struct {
	Platform       model.Platform
	PlatformPostID string
	AuthorHandle   *string
	AuthorName     *string
	Content        string
	URL            *string
	LikeCount      int
	CommentCount   int
	ShareCount     int
	ViewCount      int
	PublishedAt    time.Time
	CrawledAt      time.Time
	Sentiment      *model.Sentiment
	SentimentScore *float32
	ModelVersion   *string
	RawPayload     json.RawMessage
	Comments       []IngestCommentInput
}

// IngestService writes crawled + (optionally already-labeled) posts and
// comments from an external worker into Postgres. It is the write-side
// counterpart to MentionService, which is read-only for the dashboard.
type IngestService interface {
	IngestPost(ctx context.Context, input IngestPostInput) (*model.Post, error)
}

type ingestService struct {
	postRepo    repository.PostRepository
	commentRepo repository.CommentRepository
}

func NewIngestService(postRepo repository.PostRepository, commentRepo repository.CommentRepository) IngestService {
	return &ingestService{postRepo: postRepo, commentRepo: commentRepo}
}

func (s *ingestService) IngestPost(ctx context.Context, in IngestPostInput) (*model.Post, error) {
	post := &model.Post{
		Platform:       in.Platform,
		PlatformPostID: in.PlatformPostID,
		AuthorHandle:   in.AuthorHandle,
		AuthorName:     in.AuthorName,
		Content:        in.Content,
		URL:            in.URL,
		LikeCount:      in.LikeCount,
		CommentCount:   in.CommentCount,
		ShareCount:     in.ShareCount,
		ViewCount:      in.ViewCount,
		Sentiment:      in.Sentiment,
		SentimentScore: in.SentimentScore,
		ModelVersion:   in.ModelVersion,
		PublishedAt:    in.PublishedAt,
		CrawledAt:      in.CrawledAt,
		RawPayload:     in.RawPayload,
	}

	// Upsert never overwrites sentiment columns on conflict (see
	// repository.PostRepository.Upsert) — it only sets them on first insert.
	// Re-ingesting the same post later just refreshes engagement counters.
	if err := s.postRepo.Upsert(ctx, post); err != nil {
		return nil, err
	}

	for _, c := range in.Comments {
		comment := &model.Comment{
			PostID:            post.ID,
			PlatformCommentID: c.PlatformCommentID,
			AuthorHandle:      c.AuthorHandle,
			Content:           c.Content,
			LikeCount:         c.LikeCount,
			Sentiment:         c.Sentiment,
			SentimentScore:    c.SentimentScore,
			ModelVersion:      c.ModelVersion,
			PublishedAt:       c.PublishedAt,
			CrawledAt:         in.CrawledAt,
			RawPayload:        c.RawPayload,
		}
		if err := s.commentRepo.Upsert(ctx, comment); err != nil {
			return nil, err
		}
	}

	return post, nil
}
