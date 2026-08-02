package service

import (
	"context"
	"errors"
	"time"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
	"gorm.io/gorm"
)

var ErrPostNotFound = errors.New("post not found")

type ListMentionsInput struct {
	Platform  model.Platform  // "" = no filter
	Sentiment model.Sentiment // "" = no filter
	Query     string
	Page      int
	PerPage   int
	// From/To are nil when the frontend's topbar date-range picker is
	// cleared ("view without a date range") — every downstream query below
	// treats that as all-time, unfiltered by date.
	From *time.Time
	To   *time.Time
}

// MentionsPage bundles a page of labeled posts with the aggregates the
// /mentions dashboard endpoint needs — metrics and platform_volume must
// reflect the whole filtered set, not just the current page, so they're
// computed here rather than derived from Posts in the handler.
type MentionsPage struct {
	Posts           []model.Post
	Total           int64 // count matching the filter (drives pagination)
	TotalUnfiltered int64
	Aggregate       repository.PostAggregate
	// TotalComments is real comments (comments table), matching the same
	// platform/sentiment/query filter as Posts — distinct from
	// Aggregate.TotalEngagement, which is like/comment/share counters
	// stored on the posts themselves.
	TotalComments  int64
	PlatformVolume []repository.PlatformVolumeRow
	// PlatformMentionCounts is the real per-platform post+comment count
	// matching the current filter (empty filter = everything) — distinct
	// from PlatformVolume[].Value, which is engagement, not a count.
	PlatformMentionCounts map[model.Platform]int64
}

// MentionService serves the crawled + sentiment-labeled posts/comments to the
// dashboard. It is read-facing only — posts and comments (with sentiment
// already attached, if labeled) are written by the Python labeling worker via
// IngestService (see internal/handler/ingest.go), not by this service.
type MentionService interface {
	ListMentions(ctx context.Context, input ListMentionsInput) (MentionsPage, error)
	GetPost(ctx context.Context, id uint) (*model.Post, error)
	ListComments(ctx context.Context, postID uint) ([]model.Comment, error)
}

type mentionService struct {
	postRepo    repository.PostRepository
	commentRepo repository.CommentRepository
}

func NewMentionService(postRepo repository.PostRepository, commentRepo repository.CommentRepository) MentionService {
	return &mentionService{postRepo: postRepo, commentRepo: commentRepo}
}

func (s *mentionService) ListMentions(ctx context.Context, input ListMentionsInput) (MentionsPage, error) {
	filter := repository.PostFilter{
		Platform:  string(input.Platform),
		Sentiment: string(input.Sentiment),
		Query:     input.Query,
		From:      input.From,
		To:        input.To,
		Page:      input.Page,
		PerPage:   input.PerPage,
	}

	posts, total, err := s.postRepo.List(ctx, filter)
	if err != nil {
		return MentionsPage{}, err
	}

	agg, err := s.postRepo.Aggregate(ctx, filter)
	if err != nil {
		return MentionsPage{}, err
	}

	totalComments, err := s.commentRepo.CountFiltered(ctx, repository.CommentFilter{
		Platform:  filter.Platform,
		Sentiment: filter.Sentiment,
		Query:     filter.Query,
		From:      filter.From,
		To:        filter.To,
	})
	if err != nil {
		return MentionsPage{}, err
	}

	totalUnfiltered, err := s.postRepo.CountAll(ctx)
	if err != nil {
		return MentionsPage{}, err
	}

	platformVolume, err := s.postRepo.PlatformVolume(ctx, input.From, input.To)
	if err != nil {
		return MentionsPage{}, err
	}

	// Always computed now (post+comment counts) — no longer gated behind
	// "is a filter active", since it's the real count backing the
	// per-platform "N mentions" label, not just a secondary filtered stat.
	mentionCounts, err := s.postRepo.CombinedPlatformMentionCounts(ctx, filter)
	if err != nil {
		return MentionsPage{}, err
	}

	return MentionsPage{
		Posts:                 posts,
		Total:                 total,
		TotalUnfiltered:       totalUnfiltered,
		Aggregate:             agg,
		TotalComments:         totalComments,
		PlatformVolume:        platformVolume,
		PlatformMentionCounts: mentionCounts,
	}, nil
}

func (s *mentionService) GetPost(ctx context.Context, id uint) (*model.Post, error) {
	post, err := s.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	if post.Sentiment == nil {
		// Not yet labeled by the nightly worker — treat as not found for the
		// mentions dashboard API, which only deals in labeled data.
		return nil, ErrPostNotFound
	}
	return post, nil
}

func (s *mentionService) ListComments(ctx context.Context, postID uint) ([]model.Comment, error) {
	return s.commentRepo.ListByPostID(ctx, postID)
}
