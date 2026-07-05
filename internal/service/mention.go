package service

import (
	"context"
	"errors"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
	"gorm.io/gorm"
)

var ErrPostNotFound = errors.New("post not found")

type ListPostsInput struct {
	Platform  string
	Sentiment string
	Category  string
	Query     string
	Page      int
	PerPage   int
}

// MentionService serves the crawled + sentiment-labeled posts/comments to the
// dashboard. Writing sentiment labels back is done by the nightly labeling
// worker directly against Postgres — this service is read-facing only.
type MentionService interface {
	ListPosts(ctx context.Context, input ListPostsInput) ([]model.Post, int64, error)
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

func (s *mentionService) ListPosts(ctx context.Context, input ListPostsInput) ([]model.Post, int64, error) {
	return s.postRepo.List(ctx, repository.PostFilter{
		Platform:  input.Platform,
		Sentiment: input.Sentiment,
		Category:  input.Category,
		Query:     input.Query,
		Page:      input.Page,
		PerPage:   input.PerPage,
	})
}

func (s *mentionService) GetPost(ctx context.Context, id uint) (*model.Post, error) {
	post, err := s.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return post, nil
}

func (s *mentionService) ListComments(ctx context.Context, postID uint) ([]model.Comment, error) {
	return s.commentRepo.ListByPostID(ctx, postID)
}
